package nut

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type UPS struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type NutData struct {
	UPSList   []UPS             `json:"upsList"`
	Variables map[string]string `json:"variables"`
}

type Client struct {
	host     string
	port     string
	username string
	password string
}

func NewClientFromEnv() (*Client, error) {
	host := "192.168.1.1"
	port := "3493"
	username := os.Getenv("NUT_USERNAME")
	password := os.Getenv("NUT_PASSWORD")

	if username == "" || password == "" {
		return nil, fmt.Errorf("NUT_USERNAME and NUT_PASSWORD are required")
	}

	return &Client{
		host:     host,
		port:     port,
		username: username,
		password: password,
	}, nil
}

func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /goapi/nut", streamUPSData)
}

func streamUPSData(w http.ResponseWriter, r *http.Request) {
	client, err := NewClientFromEnv()
	if err != nil {
		http.Error(w, `{"error":"NUT client is not configured"}`, http.StatusInternalServerError)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":"streaming is not supported by this server"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	sendData := func() error {
		data, err := client.FetchData(r.Context())
		if err != nil {
			return err
		}

		jsonData, err := json.Marshal(data)
		if err != nil {
			return err
		}

		_, err = fmt.Fprintf(w, "event: message\ndata: %s\n\n", jsonData)
		if err != nil {
			return err
		}

		flusher.Flush()
		return nil
	}

	// Send the first UPS update immediately.
	if err := sendData(); err != nil {
		http.Error(w, `{"error":"failed to fetch NUT data"}`, http.StatusBadGateway)
		return
	}

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return

		case <-ticker.C:
			if err := sendData(); err != nil {
				errorJSON, _ := json.Marshal(map[string]string{
					"error": "failed to refresh NUT data",
				})

				_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", errorJSON)
				flusher.Flush()
			}
		}
	}
}

func (c *Client) FetchData(ctx context.Context) (*NutData, error) {
	dialer := net.Dialer{
		Timeout: 10 * time.Second,
	}

	conn, err := dialer.DialContext(
		ctx,
		"tcp",
		net.JoinHostPort(c.host, c.port),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to NUT server: %w", err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	reader := bufio.NewReader(conn)

	if err := sendCommand(conn, reader, "USERNAME "+c.username); err != nil {
		return nil, err
	}

	if err := sendCommand(conn, reader, "PASSWORD "+c.password); err != nil {
		return nil, err
	}

	upsLines, err := sendListCommand(conn, reader, "LIST UPS")
	if err != nil {
		return nil, err
	}

	upsList := parseUPSList(upsLines)

	variablesLines, err := sendListCommand(conn, reader, "LIST VAR CyberPower")
	if err != nil {
		return nil, err
	}

	return &NutData{
		UPSList:   upsList,
		Variables: parseVariables(variablesLines),
	}, nil
}

func sendCommand(conn net.Conn, reader *bufio.Reader, command string) error {
	if _, err := fmt.Fprintf(conn, "%s\n", command); err != nil {
		return err
	}

	line, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	line = strings.TrimSpace(line)

	if line != "OK" {
		return fmt.Errorf("NUT command %q failed: %s", command, line)
	}

	return nil
}

func sendListCommand(conn net.Conn, reader *bufio.Reader, command string) ([]string, error) {
	if _, err := fmt.Fprintf(conn, "%s\n", command); err != nil {
		return nil, err
	}

	var lines []string

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "ERR") {
			return nil, fmt.Errorf("NUT command %q failed: %s", command, line)
		}

		if strings.HasPrefix(line, "END LIST") {
			return lines, nil
		}

		if strings.HasPrefix(line, "BEGIN LIST") {
			continue
		}

		lines = append(lines, line)
	}
}

func parseUPSList(lines []string) []UPS {
	upsList := make([]UPS, 0, len(lines))

	for _, line := range lines {
		parts := strings.SplitN(line, " ", 3)
		if len(parts) != 3 || parts[0] != "UPS" {
			continue
		}

		description := parseQuotedValue(parts[2])

		upsList = append(upsList, UPS{
			Name:        parts[1],
			Description: description,
		})
	}

	return upsList
}

func parseVariables(lines []string) map[string]string {
	variables := make(map[string]string)

	for _, line := range lines {
		parts := strings.SplitN(line, " ", 4)
		if len(parts) != 4 || parts[0] != "VAR" {
			continue
		}

		variableName := parts[2]
		variableValue := parseQuotedValue(parts[3])

		variables[variableName] = variableValue
	}

	return variables
}

func parseQuotedValue(value string) string {
	value = strings.TrimSpace(value)

	if unquoted, err := strconv.Unquote(value); err == nil {
		return unquoted
	}

	return strings.Trim(value, `"`)
}
