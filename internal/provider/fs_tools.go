// Copyright IBM Corp. 2021, 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func fileExists(filename string) (bool, error) {
	_, err := os.Stat(filename)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	return false, err
}

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
