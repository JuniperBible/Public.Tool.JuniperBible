package repository

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
)

// FTPClient provides FTP download functionality for SWORD repositories
type FTPClient struct {
	opts ClientOptions
	conn *ftp.ServerConn
	host string
}

// NewFTPClient creates a new FTP client
func NewFTPClient(opts ClientOptions) *FTPClient {
	if opts.Timeout == 0 {
		opts.Timeout = 60 * time.Second
	}
	return &FTPClient{opts: opts}
}

// Connect establishes a connection to an FTP server
func (c *FTPClient) Connect(ctx context.Context, host string) error {
	// Add port if not specified
	if !strings.Contains(host, ":") {
		host = host + ":21"
	}

	conn, err := ftp.Dial(host, ftp.DialWithTimeout(c.opts.Timeout))
	if err != nil {
		return fmt.Errorf("connecting to FTP server: %w", err)
	}

	// Login as anonymous
	if err := conn.Login("anonymous", "anonymous@"); err != nil {
		conn.Quit()
		return fmt.Errorf("FTP login: %w", err)
	}

	c.conn = conn
	c.host = host
	return nil
}

// Close closes the FTP connection
func (c *FTPClient) Close() error {
	if c.conn != nil {
		return c.conn.Quit()
	}
	return nil
}

// Download fetches a file from the FTP server
func (c *FTPClient) Download(ctx context.Context, path string) ([]byte, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	resp, err := c.conn.Retr(path)
	if err != nil {
		return nil, fmt.Errorf("retrieving file: %w", err)
	}
	defer resp.Close()

	data, err := io.ReadAll(resp)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	return data, nil
}

// DownloadWithProgress fetches a file with progress reporting
func (c *FTPClient) DownloadWithProgress(ctx context.Context, path string, progress ProgressFunc) ([]byte, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	// Get file size first
	size, err := c.conn.FileSize(path)
	if err != nil {
		// Size not available, just download without progress
		return c.Download(ctx, path)
	}

	resp, err := c.conn.Retr(path)
	if err != nil {
		return nil, fmt.Errorf("retrieving file: %w", err)
	}
	defer resp.Close()

	var downloaded int64
	var data []byte
	buf := make([]byte, 32*1024)

	for {
		n, err := resp.Read(buf)
		if n > 0 {
			data = append(data, buf[:n]...)
			downloaded += int64(n)
			if progress != nil {
				progress(downloaded, size)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading file: %w", err)
		}
	}

	return data, nil
}

// ListDirectory lists files in a directory
func (c *FTPClient) ListDirectory(ctx context.Context, path string) ([]string, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	entries, err := c.conn.List(path)
	if err != nil {
		return nil, fmt.Errorf("listing directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		files = append(files, entry.Name)
	}

	return files, nil
}

// DownloadFromFTP is a convenience function that handles the full FTP download flow
func DownloadFromFTP(ctx context.Context, host, path string, opts ClientOptions) ([]byte, error) {
	client := NewFTPClient(opts)

	if err := client.Connect(ctx, host); err != nil {
		return nil, err
	}
	defer client.Close()

	return client.Download(ctx, path)
}
