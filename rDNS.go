package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	flags "github.com/jessevdk/go-flags"
)

var opts struct {
	Threads      int    `short:"t" long:"threads" default:"100" description:"How many threads should be used (max 10000)"`
	ResolverIP   string `short:"r" long:"resolver" description:"IP of the DNS resolver to use for lookups"`
	ResolverFile string `short:"R" long:"resolvers-file" description:"File containing list of DNS resolvers to use for lookups"`
	UseDefault   bool   `short:"U" long:"use-default" description:"Use default resolvers for lookups"`
	Protocol     string `short:"P" long:"protocol" choice:"tcp" choice:"udp" default:"udp" description:"Protocol to use for lookups"`
	Port         uint16 `short:"p" long:"port" default:"53" description:"Port to bother the specified DNS resolver on"`
	Domain       bool   `short:"d" long:"domain" description:"Output only domains"`
	ListFile     string `short:"l" long:"list" description:"File containing IP addresses or CIDR ranges"`
	Timeout      int    `short:"T" long:"timeout" default:"2" description:"DNS query timeout in seconds"`
	Retries      int    `short:"y" long:"retries" default:"1" description:"Number of retries per resolver"`
	Verbose      bool   `short:"v" long:"verbose" description:"Show progress and statistics"`
	Output       string `short:"o" long:"output" description:"Output file (default: stdout)"`
	ShowFailed   bool   `short:"f" long:"show-failed" description:"Show failed/unresolved IPs"`
	RateLimit    int    `short:"L" long:"rate-limit" default:"0" description:"Rate limit in queries per second (0 = no limit)"`
	Help         bool   `short:"h" long:"help" description:"Show help message"`
}

var defaultResolvers = []string{
	"1.1.1.1", "1.0.0.1", "8.8.8.8", "8.8.4.4", "9.9.9.9", "149.112.112.112",
	"208.67.222.222", "208.67.220.220", "64.6.64.6", "64.6.65.6", "198.101.242.72",
	"23.253.163.53", "8.26.56.26", "8.20.247.20", "185.228.168.9", "185.228.169.9",
	"76.76.19.19", "76.223.122.150", "94.140.14.14", "94.140.15.15",
}

type Stats struct {
	total     int64
	resolved  int64
	failed    int64
	processed int64
}

var stats Stats

func main() {
	parser := flags.NewParser(&opts, flags.Default)
	_, err := parser.Parse()

	if err != nil {
		os.Exit(1)
	}

	if opts.Help {
		parser.WriteHelp(os.Stdout)
		fmt.Println("\nExamples:")
		fmt.Println("  go run program.go -l iprange.txt -t 5000 -U")
		fmt.Println("  go run program.go -l ips.txt -t 1000 -r 8.8.8.8 -v")
		fmt.Println("  echo '192.168.1.0/24' | go run program.go -t 500 -U -d")
		os.Exit(0)
	}

	// Validate thread count
	if opts.Threads > 10000 {
		fmt.Fprintf(os.Stderr, "Warning: Thread count limited to 10000 for system stability\n")
		opts.Threads = 10000
	}

	// Setup resolvers
	var resolvers []string
	if opts.ResolverFile != "" {
		resolvers = loadResolversFromFile(opts.ResolverFile)
	}

	if opts.ResolverIP != "" {
		resolvers = append(resolvers, opts.ResolverIP)
	}

	if opts.UseDefault {
		resolvers = append(resolvers, defaultResolvers...)
	}

	if len(resolvers) == 0 {
		fmt.Fprintf(os.Stderr, "Error: No DNS resolvers specified. Use -r, -R, or -U\n")
		os.Exit(1)
	}

	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "Using %d resolvers with %d threads\n", len(resolvers), opts.Threads)
	}

	// Setup output
	var outputFile *os.File
	if opts.Output != "" {
		outputFile, err = os.Create(opts.Output)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create output file: %v\n", err)
			os.Exit(1)
		}
		defer outputFile.Close()
	} else {
		outputFile = os.Stdout
	}

	// Setup rate limiting
	var rateLimiter <-chan time.Time
	if opts.RateLimit > 0 {
		ticker := time.NewTicker(time.Second / time.Duration(opts.RateLimit))
		defer ticker.Stop()
		rateLimiter = ticker.C
	}

	// Create work channel with buffer
	work := make(chan string, opts.Threads*2)
	
	// Start progress reporter if verbose
	var progressDone chan bool
	if opts.Verbose {
		progressDone = make(chan bool)
		go showProgress(progressDone)
	}

	// Start IP generator
	go func() {
		defer close(work)
		
		if opts.ListFile != "" {
			generateIPsFromFile(opts.ListFile, work)
		} else {
			generateIPsFromStdin(work)
		}
	}()

	// Start workers
	wg := &sync.WaitGroup{}
	for i := 0; i < opts.Threads; i++ {
		wg.Add(1)
		go doWork(work, wg, resolvers, outputFile, rateLimiter)
	}

	wg.Wait()

	if opts.Verbose {
		progressDone <- true
		fmt.Fprintf(os.Stderr, "\nCompleted: %d total, %d resolved, %d failed\n", 
			atomic.LoadInt64(&stats.total), 
			atomic.LoadInt64(&stats.resolved), 
			atomic.LoadInt64(&stats.failed))
	}
}

func loadResolversFromFile(filename string) []string {
	file, err := os.Open(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open resolvers file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	var resolvers []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			resolvers = append(resolvers, line)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read resolvers file: %v\n", err)
		os.Exit(1)
	}

	return resolvers
}

func generateIPsFromFile(filename string, work chan<- string) {
	file, err := os.Open(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open input file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		expandIPRange(line, work)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read input file: %v\n", err)
		os.Exit(1)
	}
}

func generateIPsFromStdin(work chan<- string) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		expandIPRange(line, work)
	}
}

func expandIPRange(input string, work chan<- string) {
	input = strings.TrimSpace(input)
	
	// Check if it's a CIDR range
	if strings.Contains(input, "/") {
		_, ipnet, err := net.ParseCIDR(input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid CIDR range: %s\n", input)
			return
		}
		
		// Generate all IPs in the CIDR range
		for ip := ipnet.IP.Mask(ipnet.Mask); ipnet.Contains(ip); incrementIP(ip) {
			atomic.AddInt64(&stats.total, 1)
			work <- ip.String()
		}
	} else {
		// Single IP address
		if net.ParseIP(input) != nil {
			atomic.AddInt64(&stats.total, 1)
			work <- input
		} else {
			fmt.Fprintf(os.Stderr, "Invalid IP address: %s\n", input)
		}
	}
}

func incrementIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func doWork(work <-chan string, wg *sync.WaitGroup, resolvers []string, outputFile *os.File, rateLimiter <-chan time.Time) {
	defer wg.Done()

	outputMutex := &sync.Mutex{}

	for ip := range work {
		// Apply rate limiting if configured
		if rateLimiter != nil {
			<-rateLimiter
		}

		resolved := false

		for _, resolverIP := range resolvers {
			for retry := 0; retry <= opts.Retries; retry++ {
				ctx, cancel := context.WithTimeout(context.Background(), time.Duration(opts.Timeout)*time.Second)
				
				r := &net.Resolver{
					PreferGo: true,
					Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
						d := net.Dialer{
							Timeout: time.Duration(opts.Timeout) * time.Second,
						}
						return d.DialContext(ctx, opts.Protocol, fmt.Sprintf("%s:%d", resolverIP, opts.Port))
					},
				}

				addr, err := r.LookupAddr(ctx, ip)
				cancel()

				if err == nil && len(addr) > 0 {
					outputMutex.Lock()
					for _, a := range addr {
						if opts.Domain {
							fmt.Fprintln(outputFile, strings.TrimRight(a, "."))
						} else {
							fmt.Fprintf(outputFile, "%s\t%s\n", ip, strings.TrimRight(a, "."))
						}
					}
					outputMutex.Unlock()
					
					resolved = true
					atomic.AddInt64(&stats.resolved, 1)
					break
				}
				
				// Small delay between retries
				if retry < opts.Retries {
					time.Sleep(100 * time.Millisecond)
				}
			}
			
			if resolved {
				break
			}
		}

		if !resolved {
			atomic.AddInt64(&stats.failed, 1)
			if opts.ShowFailed {
				outputMutex.Lock()
				fmt.Fprintf(outputFile, "%s\tFAILED\n", ip)
				outputMutex.Unlock()
			}
		}

		atomic.AddInt64(&stats.processed, 1)
	}
}

func showProgress(done <-chan bool) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	startTime := time.Now()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			processed := atomic.LoadInt64(&stats.processed)
			resolved := atomic.LoadInt64(&stats.resolved)
			total := atomic.LoadInt64(&stats.total)
			
			elapsed := time.Since(startTime)
			rate := float64(processed) / elapsed.Seconds()
			
			fmt.Fprintf(os.Stderr, "Progress: %d/%d processed, %d resolved, %.1f IPs/sec\n", 
				processed, total, resolved, rate)
		}
	}
}
