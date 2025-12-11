#!/bin/bash

set -e

INSTALL_DIR="/usr/local/bin"
SERVICE_USER="prometheus"
CONFIG_DIR="/etc/default"

print_info() {
    echo "[INFO] $1"
}

print_warn() {
    echo "[WARN] $1"
}

print_error() {
    echo "[ERROR] $1"
}

check_root() {
    if [ "$EUID" -ne 0 ]; then
        print_error "This script must be run as root"
        exit 1
    fi
}

check_dependencies() {
    print_info "Checking dependencies..."
    
    if ! command -v go &> /dev/null; then
        print_error "Go is not installed. Please install Go 1.21+ first."
        exit 1
    fi
    
    GO_VERSION=$(go version | grep -oP 'go\d+\.\d+' | head -1)
    print_info "Found Go version: $GO_VERSION"
}

create_user() {
    print_info "Creating service user: $SERVICE_USER"
    
    if id "$SERVICE_USER" &>/dev/null; then
        print_warn "User $SERVICE_USER already exists"
    else
        useradd --system --no-create-home --shell /bin/false $SERVICE_USER
        print_info "Created user: $SERVICE_USER"
    fi
}

build_binary() {
    print_info "Building elasticsearch-shard-exporter..."
    
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    cd "$SCRIPT_DIR"
    
    go mod download
    go mod tidy
    
    BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
    VERSION=$(cat VERSION 2>/dev/null || echo "1.0.0")
    
    CGO_ENABLED=0 go build -ldflags "-s -w -X main.Version=$VERSION -X main.BuildTime=$BUILD_TIME" \
        -o elasticsearch-shard-exporter .
    
    print_info "Build complete"
}

install_binary() {
    print_info "Installing binary to $INSTALL_DIR..."
    
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    
    cp "$SCRIPT_DIR/elasticsearch-shard-exporter" "$INSTALL_DIR/"
    chmod 755 "$INSTALL_DIR/elasticsearch-shard-exporter"
    
    print_info "Binary installed: $INSTALL_DIR/elasticsearch-shard-exporter"
}

install_service() {
    print_info "Installing systemd service..."
    
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    
    cp "$SCRIPT_DIR/systemd/elasticsearch-shard-exporter.service" /etc/systemd/system/
    
    if [ ! -f "$CONFIG_DIR/elasticsearch-shard-exporter" ]; then
        cp "$SCRIPT_DIR/systemd/elasticsearch-shard-exporter.default" "$CONFIG_DIR/elasticsearch-shard-exporter"
        chmod 600 "$CONFIG_DIR/elasticsearch-shard-exporter"
        print_info "Configuration file created: $CONFIG_DIR/elasticsearch-shard-exporter"
    else
        print_warn "Configuration file already exists, skipping"
    fi
    
    systemctl daemon-reload
    
    print_info "Systemd service installed"
}

configure_service() {
    print_info "Configuring service..."
    
    echo ""
    echo "Please configure the exporter by editing: $CONFIG_DIR/elasticsearch-shard-exporter"
    echo ""
    echo "Example configuration:"
    echo "  ES_URL=http://your-elasticsearch:9200"
    echo "  ES_USER=elastic"
    echo "  ES_PASS=your-password"
    echo ""
}

enable_service() {
    print_info "Enabling service..."
    
    systemctl enable elasticsearch-shard-exporter
    print_info "Service enabled to start on boot"
}

start_service() {
    print_info "Starting service..."
    
    systemctl start elasticsearch-shard-exporter
    sleep 2
    
    if systemctl is-active --quiet elasticsearch-shard-exporter; then
        print_info "Service started successfully"
        print_info "Metrics available at: http://localhost:9061/metrics"
    else
        print_error "Service failed to start. Check logs with: journalctl -u elasticsearch-shard-exporter"
    fi
}

show_status() {
    echo "Installation Details"
    echo "  Start:   systemctl start elasticsearch-shard-exporter"
    echo "  Stop:    systemctl stop elasticsearch-shard-exporter"
    echo "  Status:  systemctl status elasticsearch-shard-exporter"
    echo "  Logs:    journalctl -u elasticsearch-shard-exporter -f"
    echo "Configuration: $CONFIG_DIR/elasticsearch-shard-exporter"
    echo "Binary:        $INSTALL_DIR/elasticsearch-shard-exporter"
    echo "Metrics:       http://localhost:9061/metrics"
}

uninstall() {
    print_info "Uninstalling elasticsearch-shard-exporter..."
    
    systemctl stop elasticsearch-shard-exporter 2>/dev/null || true
    systemctl disable elasticsearch-shard-exporter 2>/dev/null || true
    
    rm -f /etc/systemd/system/elasticsearch-shard-exporter.service
    rm -f "$INSTALL_DIR/elasticsearch-shard-exporter"
    
    systemctl daemon-reload
    
    print_info "Uninstallation complete"
    print_warn "Configuration file preserved at: $CONFIG_DIR/elasticsearch-shard-exporter"
}

usage() {
    echo "Usage: $0 [COMMAND]"
    echo ""
    echo "Commands:"
    echo "  install     Install the exporter (default)"
    echo "  uninstall   Uninstall the exporter"
    echo "  build       Build only (no installation)"
    echo "  help        Show this help message"
    echo ""
}

# Main
main() {
    case "${1:-install}" in
        install)
            check_root
            check_dependencies
            create_user
            build_binary
            install_binary
            install_service
            configure_service
            enable_service
            start_service
            show_status
            ;;
        uninstall)
            check_root
            uninstall
            ;;
        build)
            check_dependencies
            build_binary
            print_info "Binary built: ./elasticsearch-shard-exporter"
            ;;
        help|--help|-h)
            usage
            ;;
        *)
            print_error "Unknown command: $1"
            usage
            exit 1
            ;;
    esac
}

main "$@"
