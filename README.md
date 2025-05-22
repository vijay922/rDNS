# rDNS

A high-performance, concurrent reverse DNS lookup tool written in Go. Efficiently resolves IP addresses and CIDR ranges to domain names using multiple DNS resolvers and thousands of concurrent threads.

<img src="https://github.com/vijay922/rDNS/blob/main/logo.png?raw=true" alt="GoXSScanner" width="600"/>

## Features

- 🚀 **High Concurrency**: Support for up to 10,000 concurrent threads
- 🌐 **CIDR Range Support**: Automatically expands CIDR ranges (e.g., `192.168.1.0/24`)
- 🔄 **Multiple DNS Resolvers**: Use custom resolvers or built-in public DNS servers
- ⚡ **Performance Optimized**: Built-in rate limiting, timeouts, and retry mechanisms
- 📊 **Progress Tracking**: Real-time statistics and progress reporting
- 🎯 **Flexible Output**: Output to file or stdout, domains only or IP-domain pairs
- 🛡️ **Robust Error Handling**: Configurable timeouts, retries, and failure reporting

## Installation

### Quick Run
```bash
go install github.com/vijay922/rdns@latest

```

## Usage

### Basic Usage
```bash
# Resolve IPs from file using default resolvers
rdns -l iprange.txt -U

# High-performance bulk resolution
rdns -l iprange.txt -t 5000 -U -v

# Pipe single IP or CIDR
echo "8.8.8.8" | rdns -U
echo "192.168.1.0/24" | rdns -U -t 1000
```

### Advanced Usage
```bash
# Save results to file with progress
rdns -l iprange.txt -t 5000 -U -v -o results.txt

# Use custom DNS resolver with TCP
rdns -l iprange.txt -r 1.1.1.1 -P tcp -t 2000

# Show only domain names, include failed IPs
rdns -l iprange.txt -U -d -f -t 3000

# Rate limited with custom timeout and retries
rdns -l iprange.txt -U -t 5000 -L 1000 -T 5 -y 3 -v
```

## Command Line Options

| Flag | Long Flag | Default | Description |
|------|-----------|---------|-------------|
| `-t` | `--threads` | 100 | Number of concurrent threads (max 10000) |
| `-l` | `--list` | - | File containing IP addresses or CIDR ranges |
| `-r` | `--resolver` | - | Single DNS resolver IP address |
| `-R` | `--resolvers-file` | - | File containing list of DNS resolvers |
| `-U` | `--use-default` | false | Use built-in public DNS resolvers |
| `-P` | `--protocol` | udp | Protocol to use (tcp/udp) |
| `-p` | `--port` | 53 | DNS server port |
| `-T` | `--timeout` | 2 | DNS query timeout in seconds |
| `-y` | `--retries` | 1 | Number of retries per resolver |
| `-d` | `--domain` | false | Output only domain names |
| `-v` | `--verbose` | false | Show progress and statistics |
| `-o` | `--output` | stdout | Output file path |
| `-f` | `--show-failed` | false | Show failed/unresolved IPs |
| `-L` | `--rate-limit` | 0 | Rate limit in queries per second (0 = no limit) |
| `-h` | `--help` | - | Show help message |

## Input File Formats

### IP List File (`iplist.txt`)
```
# Single IP addresses
8.8.8.8
1.1.1.1
208.67.222.222

# CIDR ranges (automatically expanded)
192.168.1.0/24
10.0.0.0/16
172.16.0.0/12

# Comments are ignored
# 203.0.113.0/24
```

### DNS Resolvers File (`resolvers.txt`)
```
1.1.1.1
8.8.8.8
9.9.9.9
208.67.222.222
# Custom resolver
192.168.1.1
```

## Built-in DNS Resolvers

rDNS includes popular public DNS resolvers:
- **Cloudflare**: 1.1.1.1, 1.0.0.1
- **Google**: 8.8.8.8, 8.8.4.4
- **Quad9**: 9.9.9.9, 149.112.112.112
- **OpenDNS**: 208.67.222.222, 208.67.220.220
- **Verisign**: 64.6.64.6, 64.6.65.6
- And more...

## Performance Tuning

### System Limits
For high thread counts, you may need to increase system limits:
```bash
# Increase file descriptor limit
ulimit -n 65536

# For permanent changes, edit /etc/security/limits.conf
echo "* soft nofile 65536" >> /etc/security/limits.conf
echo "* hard nofile 65536" >> /etc/security/limits.conf
```

### Optimal Settings
```bash
# Fast scanning (recommended for most use cases)
rdns -l iprange.txt -t 2000 -U -v -L 5000

# Maximum performance (use with caution)
rdns -l iprange.txt -t 5000 -U -v -T 1 -y 1

# Conservative (for slower networks)
rdns -l iprange.txt -t 500 -U -v -T 5 -y 2 -L 1000
```

## Output Examples

### Standard Output
```
8.8.8.8         dns.google.
1.1.1.1         one.one.one.one.
208.67.222.222  resolver1.opendns.com.
```

### Domain-only Output (`-d`)
```
dns.google
one.one.one.one
resolver1.opendns.com
```

### With Failed IPs (`-f`)
```
8.8.8.8         dns.google.
192.168.1.1     FAILED
1.1.1.1         one.one.one.one.
```

## Examples

### Basic Reconnaissance
```bash
# Resolve a /24 network
echo "203.0.113.0/24" | ./rdns -U -t 1000 -v

# Bulk process multiple CIDR ranges
rdns -l networks.txt -U -t 3000 -v -o resolved_hosts.txt
```

### Security Research
```bash
# Find live hosts with reverse DNS
rdns -l target_ranges.txt -U -t 5000 -d -v -o domains.txt

# Check specific organization's IP space
echo "8.8.8.0/24" | ./rdns -U -t 500 -v
```

### Infrastructure Mapping
```bash
# Map cloud provider IP ranges
rdns -l aws_ranges.txt -U -t 2000 -v -o aws_hosts.txt

# Include failed resolutions for complete mapping
rdns -l datacenter_ips.txt -U -t 1000 -f -o complete_scan.txt
```

## Troubleshooting

### Common Issues

**1. Too many open files**
```bash
ulimit -n 65536
```

**2. DNS queries timing out**
```bash
# Increase timeout and retries
rdns -l iprange.txt -U -T 5 -y 3
```

**3. Rate limiting by DNS servers**
```bash
# Add rate limiting
rdns -l iprange.txt -U -L 1000
```

**4. No results**
```bash
# Check with verbose output
rdns -l iprange.txt -U -v -f
```

### Performance Issues
- Start with 1000 threads and increase gradually
- Use rate limiting (`-L`) for large scans
- Consider using TCP (`-P tcp`) for better reliability
- Monitor system resources during large scans

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

### Development Setup
```bash
git clone https://github.com/vijay922/rDNS.git
cd rDNS
go mod init rdns
go get github.com/jessevdk/go-flags
```

### Running Tests
```bash
go test ./...
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Built with [go-flags](https://github.com/jessevdk/go-flags) for command-line parsing
- Inspired by the need for high-performance network reconnaissance tools and **Hakrevdns**

## Disclaimer

This tool is intended for legitimate network administration, security research, and authorized penetration testing. Users are responsible for ensuring they have proper authorization before scanning networks they do not own.

<h2 id="donate" align="center">⚡️ Support</h2>

<details>
<summary>☕ Buy Me A Coffee</summary>

<p align="center">
  <a href="https://buymeacoffee.com/vijay922">
    <img src="https://img.shields.io/badge/Buy%20Me%20a%20Coffee-ffdd00?style=for-the-badge&logo=buy-me-a-coffee&logoColor=black"/>
  </a>
</p>

</details>

<p align="center">
  <b><i>"Keep pushing forward. Never surrender."</i></b>
</p>

<p align="center">🌱</p>

## Author
[chippa vijay kumar](https://github.com/vijay922)
