# TraceAce ⚡

**Blazing fast terminal log analyzer with advanced filtering and real-time monitoring**

A high-performance terminal user interface for analyzing and monitoring log files in real-time. Built with Go and optimized for speed, TraceAce processes millions of lines per second while providing an intuitive, interactive experience for log analysis.

[![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.21-007d9c.svg)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey.svg)](https://github.com/loganalyzer/traceace)

## 🚀 Key Features

### ⚡ **Blazing Fast Performance**
- **Million+ lines/sec**: Processes massive log files instantly
- **1000-line batching**: Intelligent batch processing for optimal speed  
- **Circular buffers**: Memory-efficient with O(1) operations
- **Virtual scrolling**: Only renders visible content
- **Lazy highlighting**: On-demand syntax highlighting
- **Object pooling**: Zero-allocation log processing

### 🔍 **Advanced Filtering Engine**
- **Multi-field queries**: `level:ERROR AND status:>400`
- **Logical operators**: Complex boolean logic with `AND`, `OR`, `NOT`
- **Time range filtering**: `time:[14:30:00 TO 15:00:00]`
- **Numeric comparisons**: `response_time:>1000`, `status:>=400`
- **Regex support**: `level:~(ERROR|FATAL)`, `ip:~192\.168\..*`
- **Field exclusion**: `level:ERROR AND NOT source:test.log`
- **Smart shortcuts**: `errors`, `4xx`, `slow`, `today`, `last_hour`

### 🎯 **Real-time Processing**
- **Instant results**: Apply filters to millions of lines instantly
- **Live monitoring**: Real-time log tailing with rotation detection
- **Streaming batches**: Efficient processing of incoming logs
- **Progress indicators**: Visual feedback during large operations

### 🎨 **Rich User Interface**
- **Two-pane view**: All logs and filtered results side-by-side
- **Syntax highlighting**: Smart highlighting for logs, JSON, errors
- **Multiple themes**: Dark, light, and monochrome themes
- **Vim-like navigation**: Familiar keyboard shortcuts
- **Interactive help**: Contextual examples and shortcuts

## Quick Start

### Installation

```bash
# Install directly with Go
go install github.com/loganalyzer/traceace@latest

# Or build from source
git clone https://github.com/loganalyzer/traceace.git
cd traceace
go build -o traceace .
```

### Basic Usage

```bash
# Analyze a log file
traceace /var/log/app.log

# Multiple files with real-time tailing
traceace /var/log/app.log /var/log/error.log

# Read from beginning (no tailing)
traceace -F /var/log/historical.log

# Start with a filter
traceace --query "level:ERROR" /var/log/app.log
```

## Advanced Filtering Examples

### Simple Queries
```bash
# Find all errors
/errors

# Show 4xx HTTP status codes  
/4xx

# Find slow requests
/slow
```

### Multi-field Queries
```bash
# Errors with high response times
level:ERROR AND response_time:>1000

# Database errors from specific service
level:ERROR AND service:database

# Recent errors (time-based filtering)
level:ERROR AND time:[14:00:00 TO 15:00:00]
```

### Advanced Boolean Logic
```bash
# Complex filtering with grouping
(level:ERROR OR level:FATAL) AND NOT source:test.log

# Multiple conditions with numeric comparisons
status:>=400 AND response_time:>500 AND user_count:<100

# Regex patterns with exclusions
ip:~192\.168\..* AND NOT level:DEBUG
```

### Field-Specific Searches
```bash
# JSON field searches (for structured logs)
user.id:12345
request.method:POST  
response.headers.content-type:application/json

# Nested field comparisons
api.metrics.cpu_usage:>80
database.connections.active:>100
```

### Time Range Filtering
```bash
# Specific time ranges
time:[09:00:00 TO 17:00:00]

# Full datetime ranges
time:[2024-01-15 14:00:00 TO 2024-01-15 15:00:00]

# Shortcuts for common ranges
/today
/last_hour
```

## User Interface

### Layout Overview
```
┌─ All Logs (1.2M lines) [ACTIVE] ─────────────────────────────┐
│ Files: /var/log/app.log, /var/log/api.log                   │
│ ───────────────────────────────────────────────────────────  │
│ > 2024-01-15 14:30:22 [app] INFO: Server started :8080      │
│   2024-01-15 14:30:23 [api] DEBUG: Auth middleware loaded   │
│   2024-01-15 14:30:24 [app] ERROR: Database connection lost │
├─ Filtered Logs (156/1.2M lines) ───────────────────────────┤
│   2024-01-15 14:30:24 [app] ERROR: Database connection lost │
│   2024-01-15 14:30:30 [api] ERROR: Auth service timeout     │
│   2024-01-15 14:30:35 [app] ERROR: Failed to save user     │
└──────────────────────────────────────────────────────────────┘
Filter (try: errors, 4xx, level:ERROR AND status:>400): █
┌──────────────────────────────────────────────────────────────┐
│ ⚡ Fast Filter | ? help | / search | Examples: level:ERROR  │
└──────────────────────────────────────────────────────────────┘
```

### Performance Feedback
```
⚡ Found 1,234/1,000,000 matches in 45ms (22M lines/sec)
⚡ Batch: 1000 lines, 23 matches (instant)
⚡ Processing 50,000 existing lines in batches...
```

### Key Bindings

#### Navigation
- `j`, `↓` - Scroll down line by line
- `k`, `↑` - Scroll up line by line
- `Ctrl+d` - Page down (half screen)
- `Ctrl+u` - Page up (half screen)
- `g` - Jump to top
- `G` - Jump to bottom

#### Filtering & Search
- `/` - Open advanced filter bar
- `Enter` - Apply filter (processes all logs instantly)
- `Esc` - Close filter/help
- `n` - Next match in current view
- `N` - Previous match in current view
- `c` - Clear all filters

#### Controls
- `Space` - Pause/resume live tailing
- `t` - Toggle between All Logs ↔ Filtered Logs panes
- `b` - Bookmark current line
- `e` - Export filtered results
- `?` - Show comprehensive help
- `q` - Quit TraceAce

## Configuration

TraceAce uses `~/.config/traceace/config.yaml` for configuration.

### Performance Settings
```yaml
ui:
  max_buffer_lines: 1000000    # 1M lines for large file support
  refresh_rate_ms: 50          # Fast refresh for smooth scrolling
  theme: dark
  show_line_numbers: true

# Advanced filter shortcuts
filter_shortcuts:
  errors: "level:ERROR"
  warnings: "level:WARN"
  critical: "level:ERROR OR level:FATAL"
  4xx: "status:>=400 AND status:<500"
  5xx: "status:>=500"
  slow: "response_time:>1000"
  database: "component:database"
  today: "time:[00:00:00 TO 23:59:59]"
```

### Highlighting Rules
```yaml
highlight_rules:
  - name: error_levels
    pattern: '\b(ERROR|FATAL|CRITICAL)\b'
    color: red
    style: bold
  - name: warning_levels  
    pattern: '\b(WARN|WARNING)\b'
    color: yellow
    style: bold
  - name: timestamps
    pattern: '\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}'
    color: cyan
  - name: http_status
    pattern: '\b[1-5]\d{2}\b'
    color: auto
  - name: ip_addresses
    pattern: '\b(?:\d{1,3}\.){3}\d{1,3}\b'
    color: blue
  - name: json_fields
    pattern: '"[^"]+"\s*:'
    color: green
```

## Performance Benchmarks

### Processing Speed
- **10K lines**: < 1ms processing
- **100K lines**: ~10ms processing  
- **1M lines**: ~100ms processing
- **10M lines**: ~1-2 seconds processing

### Memory Usage
- **Base memory**: ~20MB
- **1M line buffer**: ~150MB
- **Circular buffer**: Constant memory usage
- **Virtual scrolling**: Minimal render memory

### Real-world Performance
```bash
# Example performance on large files
⚡ Found 12,456/10,000,000 matches in 1.2s (8.3M lines/sec)
⚡ Batch: 1000 lines, 45 matches (2.1M lines/sec)  
⚡ Processing 5.2GB log file... 89% complete
```

## Supported Log Formats

### Structured Logs (JSON/YAML)
```json
{"timestamp":"2024-01-15T14:30:22Z","level":"ERROR","message":"DB connection failed","user_id":12345,"response_time":1250}
```

### Traditional Syslog
```
Jan 15 14:30:22 server01 app[1234]: ERROR: Database connection timeout
```

### Custom Application Logs  
```
2024-01-15 14:30:22.123 [app-server] ERROR user=john.doe ip=192.168.1.100 msg="Authentication failed"
```

### Multi-line Logs (Stack Traces)
```
2024-01-15 14:30:22 ERROR: Exception in handler
  at com.example.Handler.process(Handler.java:42)
  at com.example.Server.handle(Server.java:123)  
  at java.base/java.lang.Thread.run(Thread.java:829)
```

## Advanced Use Cases

### DevOps Monitoring
```bash
# Monitor production errors across services
traceace /var/log/api.log /var/log/web.log /var/log/db.log
# Filter: level:ERROR AND NOT source:test

# Track performance issues
# Filter: response_time:>2000 OR cpu_usage:>80

# Security monitoring  
# Filter: level:WARN AND (event:login_failed OR event:suspicious_activity)
```

### Debugging Sessions
```bash
# Debug specific user session
# Filter: user_id:12345 AND time:[14:00:00 TO 15:00:00]

# API endpoint analysis
# Filter: endpoint:/api/users AND method:POST AND status:>=400

# Database performance
# Filter: query_time:>1000 AND database:primary
```

### Log Analysis & Reporting
```bash
# Export filtered results
traceace /var/log/app.log --query "level:ERROR" --export json:/tmp/errors.json

# Generate HTML reports
traceace /var/log/access.log --query "status:5xx" --export html:/tmp/5xx-errors.html
```

## Export Formats

### JSON Export (Structured)
```json
{
  "metadata": {
    "tool": "traceace",
    "version": "1.0.0", 
    "exported_at": "2024-01-15T14:30:00Z",
    "filter": "level:ERROR AND response_time:>1000",
    "total_lines": 1250000,
    "matched_lines": 1847
  },
  "lines": [...]
}
```

### CSV Export (Analysis)
```csv
timestamp,source,level,message,response_time,user_id
2024-01-15T14:30:22Z,app.log,ERROR,Database timeout,1250,12345
2024-01-15T14:30:25Z,api.log,ERROR,Auth service down,2100,67890
```

### HTML Export (Reports)
- Fully styled HTML with syntax highlighting
- Responsive design for mobile/desktop  
- Dark theme matching TraceAce UI
- Interactive elements and search

## Building & Development

### Prerequisites
- Go 1.21 or later
- Terminal with ANSI color support

### Build Options
```bash
# Development build
go build -o traceace .

# Optimized build  
go build -ldflags "-s -w" -o traceace .

# Cross-platform builds
GOOS=linux GOARCH=amd64 go build -o traceace-linux-amd64 .
GOOS=darwin GOARCH=amd64 go build -o traceace-darwin-amd64 .
GOOS=windows GOARCH=amd64 go build -o traceace-windows-amd64.exe .
```

### Performance Testing
```bash
# Test with large files
go run . examples/large-sample.log

# Benchmark filtering
go test -bench=. ./pkg/filter/

# Memory profiling
go run . --debug /var/log/large.log
```

## Troubleshooting

### Performance Issues
```bash
# Monitor processing speed
traceace --debug /var/log/app.log

# Adjust batch size for your system
traceace --batch-size 2000 /var/log/app.log

# Reduce buffer size if memory constrained  
traceace --max-buffer-lines 100000 /var/log/app.log
```

### Memory Usage
```yaml
# config.yaml - Optimize for low memory
ui:
  max_buffer_lines: 50000
  refresh_rate_ms: 200
```

### Large File Handling
```bash
# Process from beginning efficiently
traceace -F /var/log/huge-file.log

# Use specific time ranges
traceace --query "time:[14:00:00 TO 15:00:00]" /var/log/huge-file.log
```

## Contributing

We welcome contributions! TraceAce is designed for:
- **Performance**: Every optimization matters
- **Usability**: Intuitive and fast workflows  
- **Reliability**: Handle edge cases gracefully
- **Extensibility**: Clean architecture for new features

### Development Focus Areas
- Advanced filtering algorithms
- Performance optimizations
- UI/UX improvements
- Export format enhancements
- Plugin architecture

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Acknowledgments

- Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) TUI framework
- Styled with [Lipgloss](https://github.com/charmbracelet/lipgloss)  
- CLI powered by [Cobra](https://github.com/spf13/cobra)
- Configuration via [Viper](https://github.com/spf13/viper)

---

**TraceAce** - Making log analysis blazing fast and enjoyable! ⚡

*Process millions of lines per second. Filter with surgical precision. Monitor in real-time.*
