// Copyright IBM Corp. 2021, 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

func SHA256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()

	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

func getFileChmod(filePath string) (string, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to get file permission: %s", err.Error())
	}

	mode := info.Mode()
	perm := mode.Perm()
	perms := strings.TrimSpace(fmt.Sprintf("%4o", perm))
	return perms, nil
}

func setFileChmod(filePath string, chmod string) error {
	perm, err := strconv.ParseUint(chmod, 8, 32)
	if err != nil {
		return fmt.Errorf("failed to convert chmod permissions: %v", err)
	}
	filePerms := os.FileMode(perm)
	err = os.Chmod(filePath, filePerms)
	if err != nil {
		return fmt.Errorf("failed to set file permissions: %v", err)
	}
	return nil
}

func httpFileDownload(ctx context.Context, url, downloadPath string, headers map[string]string) (response fileChecksums, err error) {

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return response, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return response, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return response, errors.New("Unexpected http response code: " + resp.Status)
	}

	response.Etag = resp.Header.Get("ETag")
	tflog.Debug(ctx, fmt.Sprintf("ETag value = %s", response.Etag))

	if response.Etag == "" {
		return response, errors.New("failed to read Etag header value or its value is empty")
	}

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
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return response, err
	}

	sha256Sum, err := SHA256File(downloadPath)
	if err != nil {
		return response, err
	}

	response.sha256 = sha256Sum
	return response, nil
}

func getEtag(ctx context.Context, url string, headers map[string]string) (string, error) {
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return "", err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.New("failed to download file: " + resp.Status)
	}

	etag := resp.Header.Get("ETag")
	tflog.Debug(ctx, fmt.Sprintf("ETag value = %s", etag))

	if etag == "" {
		return "", errors.New("failed to read Etag header value or its value is empty")
	}
	return etag, nil
}

type fileChecksums struct {
	Etag   string
	sha256 string
}
