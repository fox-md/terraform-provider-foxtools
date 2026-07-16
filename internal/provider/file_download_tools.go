// Copyright IBM Corp. 2021, 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

func sendHttpRequest(ctx context.Context, url, method string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("Unexpected http response code. Expected 200, got: " + resp.Status)
	}

	return resp, nil
}

func checkCachingHeaders(ctx context.Context, resp *http.Response) (fileAttributes, error) {
	var response fileAttributes
	etag := parseETagHeader(ctx, resp)
	response.ETag = etag

	lastModified, err := parseLastModifiedHeader(ctx, resp)
	if err != nil {
		return response, err
	}
	response.LastModified = lastModified

	if etag == "" && lastModified == "" {
		return response, fmt.Errorf("Both ETag and LastModified headers are empty. Got Etag = '%s', LastModified = '%s'", etag, lastModified)
	}

	return response, nil
}

func parseLastModifiedHeader(ctx context.Context, resp *http.Response) (string, error) {
	lastModStr := resp.Header.Get("Last-Modified")
	if lastModStr == "" {
		tflog.Debug(ctx, "Last-Modified header not found in response")
		return "", nil
	}

	parsedTime, err := time.Parse(time.RFC1123, lastModStr)
	if err != nil {
		return "", fmt.Errorf("Error parsing date: %s", err.Error())
	}

	timestamp := parsedTime.Unix()
	tflog.Debug(ctx, fmt.Sprintf("Last-Modified unix timestamp value = '%d'", timestamp))

	return strconv.FormatInt(timestamp, 10), nil
}

func parseETagHeader(ctx context.Context, resp *http.Response) string {
	etag := resp.Header.Get("ETag")
	if etag == "" {
		tflog.Debug(ctx, "ETag header not found in response")
		return ""
	}
	tflog.Debug(ctx, fmt.Sprintf("ETag value = %s", etag))
	return etag
}

func httpFileDownload(ctx context.Context, url, downloadPath string, headers map[string]string) (fileAttributes, error) {
	var response fileAttributes

	resp, err := sendHttpRequest(ctx, url, http.MethodGet, headers)
	if err != nil {
		return response, err
	}

	defer resp.Body.Close()

	response, err = checkCachingHeaders(ctx, resp)
	if err != nil {
		return response, err
	}

	contLen := resp.Header.Get("Content-Length")
	fileSize, err := strconv.ParseInt(contLen, 10, 64)
	if err != nil {
		return response, fmt.Errorf("Error during fileSize conversion: %s", err.Error())
	}

	limited := io.LimitReader(resp.Body, fileSize+1)

	dir := filepath.Dir(downloadPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return response, err
	}

	out, err := os.Create(downloadPath)
	if err != nil {
		return response, err
	}
	defer out.Close()

	// Stream directly from network to disk
	written, err := io.Copy(out, limited)
	if err != nil {
		return response, err
	}

	if written > fileSize {
		return response, fmt.Errorf("Bytes written %d to file exceeded max file size of %d bytes", written, fileSize)
	}

	sha256Sum, err := SHA256File(downloadPath)
	if err != nil {
		return response, err
	}

	response.Sha256 = sha256Sum
	return response, nil
}

func getCachingHeaders(ctx context.Context, url string, headers map[string]string) (fileAttributes, error) {
	var response fileAttributes

	resp, err := sendHttpRequest(ctx, url, http.MethodHead, headers)
	if err != nil {
		return response, err
	}

	defer resp.Body.Close()

	response, err = checkCachingHeaders(ctx, resp)
	if err != nil {
		return response, err
	}

	return response, nil
}

type fileAttributes struct {
	ETag         string
	Sha256       string
	LastModified string
}
