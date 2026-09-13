package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	mb "github.com/momobasehq/momobase/providers"
)

func Client() *http.Client { return &http.Client{Timeout: 30 * time.Second} }

func Do(ctx context.Context, client *http.Client, method, endpoint string, headers map[string]string, contentType string, body []byte) ([]byte, error) {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, r)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	b, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		msg := strings.TrimSpace(string(b))
		if len(msg) > 500 {
			msg = msg[:500]
		}
		return nil, fmt.Errorf("upstream returned %s: %s", res.Status, mb.Redact(msg))
	}
	return b, nil
}

func JSON(ctx context.Context, client *http.Client, method, endpoint string, headers map[string]string, in, out any) error {
	var body []byte
	var err error
	if in != nil {
		body, err = json.Marshal(in)
		if err != nil {
			return err
		}
	}
	b, err := Do(ctx, client, method, endpoint, headers, "application/json", body)
	if err != nil {
		return err
	}
	if out != nil && len(bytes.TrimSpace(b)) > 0 {
		return json.Unmarshal(b, out)
	}
	return nil
}

func Form(ctx context.Context, client *http.Client, method, endpoint string, headers map[string]string, values url.Values, out any) error {
	b, err := Do(ctx, client, method, endpoint, headers, "application/x-www-form-urlencoded", []byte(values.Encode()))
	if err != nil {
		return err
	}
	if out != nil {
		return json.Unmarshal(b, out)
	}
	return nil
}

func Multipart(ctx context.Context, client *http.Client, endpoint string, headers map[string]string, fields map[string]string, out any) error {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if v != "" {
			_ = w.WriteField(k, v)
		}
	}
	_ = w.Close()
	b, err := Do(ctx, client, http.MethodPost, endpoint, headers, w.FormDataContentType(), buf.Bytes())
	if err != nil {
		return err
	}
	if out != nil {
		return json.Unmarshal(b, out)
	}
	return nil
}

func Header(headers map[string]string, name string) string {
	for k, v := range headers {
		if strings.EqualFold(k, name) {
			return v
		}
	}
	return ""
}

func Map(v any) map[string]any {
	b, _ := json.Marshal(v)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}
