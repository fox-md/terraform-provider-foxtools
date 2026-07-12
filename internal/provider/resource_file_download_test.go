// Copyright IBM Corp. 2021, 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"

	"github.com/djherbis/times"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestFileDownload(t *testing.T) {

	sha256, _ := SHA256File(ProjectRoot() + "/tests/file_download/file1.json")

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.json")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "foxtools_file_download" "test" {
  url = "http://localhost:8081/file1.json"
  filename = "` + filePath + `"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("foxtools_file_download.test", "sha256", sha256),
					// Verify dynamic values have any value set in the state.
					resource.TestCheckResourceAttrSet("foxtools_file_download.test", "download_timestamp"),
				),
			},
		},
	})
}

func TestFileDownloadCustomPermissions(t *testing.T) {

	sha256, _ := SHA256File(ProjectRoot() + "/tests/file_download/file1.json")

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.json")
	filePerm1 := "444"
	filePerm2 := "644"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "foxtools_file_download" "test" {
  url = "http://localhost:8081/file1.json"
  filename = "` + filePath + `"
  file_chmod = "` + filePerm1 + `"
}
`,
				Check: resource.TestCheckFunc(func(s *terraform.State) error {

					localPerms, err := getFileChmod(filePath)
					if err != nil {
						return err
					}

					if localPerms != filePerm1 {
						return fmt.Errorf("File permissions are not valid. Expected: '%s', Actual: '%s'", filePerm1, localPerms)
					}

					return nil
				}),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("sha256"),
						knownvalue.StringExact(sha256),
					),
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("file_chmod"),
						knownvalue.StringExact(filePerm1),
					),
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("download_timestamp"),
						knownvalue.NotNull(),
					),
				},
			},
			{
				Config: providerConfig + `
resource "foxtools_file_download" "test" {
  url = "http://localhost:8081/file1.json"
  filename = "` + filePath + `"
  file_chmod = "` + filePerm2 + `"
}
`,
				Check: resource.TestCheckFunc(func(s *terraform.State) error {
					localPerms, err := getFileChmod(filePath)
					if err != nil {
						return err
					}

					// info, err := os.Stat(filePath)
					// if err != nil {
					// 	log.Fatal(err)
					// }

					// mode := info.Mode()
					// perms := mode.Perm()
					// localPerms := strings.TrimSpace(fmt.Sprintf("%4o", perms))

					if localPerms != filePerm2 {
						return fmt.Errorf("File permissions are not valid. Expected: '%s', Actual: '%s'", filePerm2, localPerms)
					}

					return nil
				}),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("sha256"),
						knownvalue.StringExact(sha256),
					),
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("file_chmod"),
						knownvalue.StringExact(filePerm2),
					),
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("download_timestamp"),
						knownvalue.NotNull(),
					),
				},
			},
			{
				PreConfig: func() {
					err := setFileChmod(filePath, "777")
					if err != nil {
						t.Fatalf("failed to set file permissions: %v", err)
					}
				},
				Config: providerConfig + `
resource "foxtools_file_download" "test" {
  url = "http://localhost:8081/file1.json"
  filename = "` + filePath + `"
  file_chmod = "` + filePerm2 + `"
}
`,
				Check: resource.TestCheckFunc(func(s *terraform.State) error {
					localPerms, err := getFileChmod(filePath)
					if err != nil {
						return err
					}

					if localPerms != filePerm2 {
						return fmt.Errorf("File permissions are not valid. Expected: '%s', Actual: '%s'", filePerm2, localPerms)
					}

					return nil
				}),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("sha256"),
						knownvalue.StringExact(sha256),
					),
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("file_chmod"),
						knownvalue.StringExact(filePerm2),
					),
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("download_timestamp"),
						knownvalue.NotNull(),
					),
				},
			},
		},
	})
}

func TestFileDownloadWrongPermissions(t *testing.T) {

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.json")
	filePerm := "789"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "foxtools_file_download" "test" {
  url = "http://localhost:8081/file1.json"
  filename = "` + filePath + `"
  file_chmod = "` + filePerm + `"
}
`,
				ExpectError: regexp.MustCompile(`Attribute file_chmod Change mode is not valid.`),
			},
		},
	})
}

func TestFileDownloadBasicAuth(t *testing.T) {

	sha256, _ := SHA256File(ProjectRoot() + "/tests/file_download/file1.json")

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.json")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "foxtools_file_download" "test" {
  url = "http://localhost:8082/file1.json"
  filename = "` + filePath + `"
}
`,
				ExpectError: regexp.MustCompile(`Unexpected http response code: 401 Unauthorized`),
			},
			{
				Config: providerConfig + `
resource "foxtools_file_download" "test" {
  url = "http://localhost:8082/file1.json"
  filename = "` + filePath + `"
  headers = {
	"Authorization" = "Basic YWRtaW46cmVnaXN0cnkx"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("foxtools_file_download.test", "sha256", sha256),
					// Verify dynamic values have any value set in the state.
					resource.TestCheckResourceAttrSet("foxtools_file_download.test", "download_timestamp"),
				),
			},
		},
	})
}

func TestFileDownloadSetHeaders(t *testing.T) {

	var stateTimestamp string
	var localTimestamp string
	sha256, _ := SHA256File(ProjectRoot() + "/tests/file_download/file1.json")

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.json")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "foxtools_file_download" "test" {
  url = "http://localhost:8081/file1.json"
  filename = "` + filePath + `"
}
`,
				Check: resource.TestCheckFunc(func(s *terraform.State) error {
					rs := s.RootModule().Resources["foxtools_file_download.test"]

					stateTimestamp = rs.Primary.Attributes["download_timestamp"]

					t, err := times.Stat(filePath)
					if err != nil {
						return fmt.Errorf("failed to read file creation timestamp, %s", err.Error())
					}

					localTimestamp = t.ChangeTime().String()

					return nil
				}),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("sha256"),
						knownvalue.StringExact(sha256),
					),
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("download_timestamp"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("headers"),
						knownvalue.Null(),
					),
				},
			},
			{
				Config: providerConfig + `
resource "foxtools_file_download" "test" {
  url = "http://localhost:8081/file1.json"
  filename = "` + filePath + `"
  headers = {
    "User-Agent" = "terraform"
  }
}
`,
				Check: resource.TestCheckFunc(func(s *terraform.State) error {
					rs := s.RootModule().Resources["foxtools_file_download.test"]
					if stateTimestamp != rs.Primary.Attributes["download_timestamp"] {
						return fmt.Errorf("download_timestamp values do not match. Expected: %s, Actual: %s", stateTimestamp, rs.Primary.Attributes["download_timestamp"])
					}
					t, err := times.Stat(filePath)
					if err != nil {
						return fmt.Errorf("failed to read file creation timestamp, %s", err.Error())
					}

					if localTimestamp != t.ChangeTime().String() {
						return fmt.Errorf("local file changetime has been modified. Expected: %s, Actual: %s", localTimestamp, t.ChangeTime().String())
					}
					return nil
				}),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("sha256"),
						knownvalue.StringExact(sha256),
					),
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("headers"),
						knownvalue.MapExact(map[string]knownvalue.Check{
							"User-Agent": knownvalue.StringExact("terraform"),
						}),
					),
				},
			},
			{
				Config: providerConfig + `
resource "foxtools_file_download" "test" {
  url = "http://localhost:8081/file1.json"
  filename = "` + filePath + `"
}
`,
				Check: resource.TestCheckFunc(func(s *terraform.State) error {
					rs := s.RootModule().Resources["foxtools_file_download.test"]
					if stateTimestamp != rs.Primary.Attributes["download_timestamp"] {
						return fmt.Errorf("download_timestamp values do not match. Expected: %s, Actual: %s", stateTimestamp, rs.Primary.Attributes["download_timestamp"])
					}
					t, err := times.Stat(filePath)
					if err != nil {
						return fmt.Errorf("failed to read file creation timestamp, %s", err.Error())
					}

					if localTimestamp != t.ChangeTime().String() {
						return fmt.Errorf("local file changetime has been modified. Expected: %s, Actual: %s", localTimestamp, t.ChangeTime().String())
					}
					return nil
				}),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("sha256"),
						knownvalue.StringExact(sha256),
					),
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("headers"),
						knownvalue.Null(),
					),
				},
			},
		},
	})
}

func TestFileDownloadChangeURLDifferentFiles(t *testing.T) {

	var stateTimestamp string
	var localTimestamp string
	sha256_01, _ := SHA256File(ProjectRoot() + "/tests/file_download/file-01.txt")
	sha256_02, _ := SHA256File(ProjectRoot() + "/tests/file_download/file-02.txt")

	content_01, err := os.ReadFile(ProjectRoot() + "/tests/file_download/file-01.txt")
	if err != nil {
		log.Fatalf("Failed to read file: %s", err)
	}

	content_02, err := os.ReadFile(ProjectRoot() + "/tests/file_download/file-02.txt")
	if err != nil {
		log.Fatalf("Failed to read file: %s", err)
	}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.json")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "foxtools_file_download" "test" {
  url = "http://localhost:8081/file.txt"
  filename = "` + filePath + `"
}
`,
				Check: resource.TestCheckFunc(func(s *terraform.State) error {
					rs := s.RootModule().Resources["foxtools_file_download.test"]

					stateTimestamp = rs.Primary.Attributes["download_timestamp"]

					t, err := times.Stat(filePath)
					if err != nil {
						return fmt.Errorf("failed to read file creation timestamp, %s", err.Error())
					}

					localTimestamp = t.ChangeTime().String()

					content, err := os.ReadFile(filePath)
					if err != nil {
						log.Fatalf("Failed to read file: %s", err)
					}

					if string(content) != string(content_01) {
						return fmt.Errorf("file content do not match. Expected: '%s', Actual: '%s'", string(content), string(content_01))
					}

					return nil
				}),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("sha256"),
						knownvalue.StringExact(sha256_01),
					),
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("download_timestamp"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("headers"),
						knownvalue.Null(),
					),
				},
			},
			{
				Config: providerConfig + `
resource "foxtools_file_download" "test" {
  url = "http://localhost:8082/file.txt"
  filename = "` + filePath + `"
  headers = {
    "Authorization" = "Basic YWRtaW46cmVnaXN0cnkx"
  }
}
`,
				Check: resource.TestCheckFunc(func(s *terraform.State) error {
					rs := s.RootModule().Resources["foxtools_file_download.test"]
					if stateTimestamp == rs.Primary.Attributes["download_timestamp"] {
						return fmt.Errorf("download_timestamp values match. Old: %s = Current: %s", stateTimestamp, rs.Primary.Attributes["download_timestamp"])
					}

					t, err := times.Stat(filePath)
					if err != nil {
						return fmt.Errorf("failed to read file creation timestamp, %s", err.Error())
					}

					if localTimestamp == t.ChangeTime().String() {
						return fmt.Errorf("local file changetime has not changed. Old: %s = Current: %s", localTimestamp, t.ChangeTime().String())
					}

					content, err := os.ReadFile(filePath)
					if err != nil {
						log.Fatalf("Failed to read file: %s", err)
					}

					if string(content) != string(content_02) {
						return fmt.Errorf("file content do not match. Expected: '%s', Actual: '%s'", string(content), string(content_02))
					}

					return nil
				}),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("sha256"),
						knownvalue.StringExact(sha256_02),
					),
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("headers"),
						knownvalue.MapExact(map[string]knownvalue.Check{
							"Authorization": knownvalue.StringExact("Basic YWRtaW46cmVnaXN0cnkx"),
						}),
					),
				},
			},
		},
	})
}

func TestFileDownloadChangeURLsSameFile(t *testing.T) {

	var stateTimestamp string
	var localTimestamp string
	sha256, _ := SHA256File(ProjectRoot() + "/tests/file_download/file1.json")

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.json")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "foxtools_file_download" "test" {
  url = "http://localhost:8081/file1.json"
  filename = "` + filePath + `"
}
`,
				Check: resource.TestCheckFunc(func(s *terraform.State) error {
					rs := s.RootModule().Resources["foxtools_file_download.test"]

					stateTimestamp = rs.Primary.Attributes["download_timestamp"]

					t, err := times.Stat(filePath)
					if err != nil {
						return fmt.Errorf("failed to read file creation timestamp, %s", err.Error())
					}

					localTimestamp = t.ChangeTime().String()

					return nil
				}),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("sha256"),
						knownvalue.StringExact(sha256),
					),
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("download_timestamp"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("headers"),
						knownvalue.Null(),
					),
				},
			},
			{
				Config: providerConfig + `
resource "foxtools_file_download" "test" {
  url = "http://localhost:8082/file1.json"
  filename = "` + filePath + `"
  headers = {
    "Authorization" = "Basic YWRtaW46cmVnaXN0cnkx"
  }
}
`,
				Check: resource.TestCheckFunc(func(s *terraform.State) error {
					rs := s.RootModule().Resources["foxtools_file_download.test"]
					if stateTimestamp != rs.Primary.Attributes["download_timestamp"] {
						return fmt.Errorf("download_timestamp values do not match. Expected: %s, Actual: %s", stateTimestamp, rs.Primary.Attributes["download_timestamp"])
					}
					t, err := times.Stat(filePath)
					if err != nil {
						return fmt.Errorf("failed to read file creation timestamp, %s", err.Error())
					}

					if localTimestamp != t.ChangeTime().String() {
						return fmt.Errorf("local file changetime has been modified. Expected: %s, Actual: %s", localTimestamp, t.ChangeTime().String())
					}

					return nil
				}),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("sha256"),
						knownvalue.StringExact(sha256),
					),
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("headers"),
						knownvalue.MapExact(map[string]knownvalue.Check{
							"Authorization": knownvalue.StringExact("Basic YWRtaW46cmVnaXN0cnkx"),
						}),
					),
				},
			},
		},
	})
}

func TestFileDownloadChangePathDifferentFiles(t *testing.T) {

	var stateTimestamp string
	var localTimestamp string
	sha256_1, _ := SHA256File(ProjectRoot() + "/tests/file_download/file-01.txt")
	sha256_2, _ := SHA256File(ProjectRoot() + "/tests/file_download/file-02.txt")

	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()
	filePath1 := filepath.Join(tmpDir1, "test.txt")
	filePath2 := filepath.Join(tmpDir2, "test.txt")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "foxtools_file_download" "test" {
  url = "http://localhost:8081/file.txt"
  filename = "` + filePath1 + `"
}
`,
				Check: resource.TestCheckFunc(func(s *terraform.State) error {
					rs := s.RootModule().Resources["foxtools_file_download.test"]

					stateTimestamp = rs.Primary.Attributes["download_timestamp"]

					t, err := times.Stat(filePath1)
					if err != nil {
						return fmt.Errorf("failed to read file creation timestamp, %s", err.Error())
					}

					localTimestamp = t.BirthTime().String()

					return nil
				}),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("sha256"),
						knownvalue.StringExact(sha256_1),
					),
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("download_timestamp"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("headers"),
						knownvalue.Null(),
					),
				},
			},
			{
				Config: providerConfig + `
resource "foxtools_file_download" "test" {
  url = "http://localhost:8082/file.txt"
  filename = "` + filePath2 + `"
  headers = {
    "Authorization" = "Basic YWRtaW46cmVnaXN0cnkx"
  }
}
`,
				Check: resource.TestCheckFunc(func(s *terraform.State) error {
					rs := s.RootModule().Resources["foxtools_file_download.test"]
					if stateTimestamp == rs.Primary.Attributes["download_timestamp"] {
						return fmt.Errorf("download_timestamp should not match. Expected to be different, got:%s", stateTimestamp)
					}
					t, err := times.Stat(filePath2)
					if err != nil {
						return fmt.Errorf("failed to read file creation timestamp, %s", err.Error())
					}

					if localTimestamp == t.BirthTime().String() {
						return fmt.Errorf("local file birthtime has not been changed. Expected to be different, got: %s", localTimestamp)
					}

					return nil
				}),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("sha256"),
						knownvalue.StringExact(sha256_2),
					),
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("headers"),
						knownvalue.MapExact(map[string]knownvalue.Check{
							"Authorization": knownvalue.StringExact("Basic YWRtaW46cmVnaXN0cnkx"),
						}),
					),
				},
			},
		},
	})
}

func TestFileDownloadChangePathSameFile(t *testing.T) {

	var stateTimestamp string
	var localTimestamp string
	sha256, _ := SHA256File(ProjectRoot() + "/tests/file_download/file1.json")

	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()
	filePath1 := filepath.Join(tmpDir1, "test.json")
	filePath2 := filepath.Join(tmpDir2, "test.json")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "foxtools_file_download" "test" {
  url = "http://localhost:8081/file1.json"
  filename = "` + filePath1 + `"
}
`,
				Check: resource.TestCheckFunc(func(s *terraform.State) error {
					rs := s.RootModule().Resources["foxtools_file_download.test"]

					stateTimestamp = rs.Primary.Attributes["download_timestamp"]

					t, err := times.Stat(filePath1)
					if err != nil {
						return fmt.Errorf("failed to read file creation timestamp, %s", err.Error())
					}

					localTimestamp = t.BirthTime().String()

					return nil
				}),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("sha256"),
						knownvalue.StringExact(sha256),
					),
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("download_timestamp"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("headers"),
						knownvalue.Null(),
					),
				},
			},
			{
				Config: providerConfig + `
resource "foxtools_file_download" "test" {
  url = "http://localhost:8081/file1.json"
  filename = "` + filePath2 + `"
}
`,
				Check: resource.TestCheckFunc(func(s *terraform.State) error {
					rs := s.RootModule().Resources["foxtools_file_download.test"]
					if stateTimestamp != rs.Primary.Attributes["download_timestamp"] {
						return fmt.Errorf("download_timestamp values do not match. Expected: %s, Actual: %s", stateTimestamp, rs.Primary.Attributes["download_timestamp"])
					}
					t, err := times.Stat(filePath2)
					if err != nil {
						return fmt.Errorf("failed to read file creation timestamp, %s", err.Error())
					}

					if localTimestamp != t.BirthTime().String() {
						return fmt.Errorf("local file changetime has been modified. Expected: %s, Actual: %s", localTimestamp, t.BirthTime().String())
					}

					return nil
				}),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("sha256"),
						knownvalue.StringExact(sha256),
					),
					statecheck.ExpectKnownValue(
						"foxtools_file_download.test",
						tfjsonpath.New("headers"),
						knownvalue.Null(),
					),
				},
			},
		},
	})
}

func ProjectRoot() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("unable to determine caller")
	}

	// Adjust ".." depending on where this file lives.
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "../.."))
}
