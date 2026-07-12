resource "foxtools_file_download" "zip" {
  url      = "https://github.com/fox-md/terraform-provider-foxcon/releases/download/v1.0.1/terraform-provider-foxcon_1.0.1_darwin_arm64.zip"
  filename = "terraform-provider-foxcon_1.0.1_darwin_arm64.zip"
}

resource "foxtools_file_download" "json" {
  url      = "http://localhost/file.json"
  filename = "/tmp/file.json"
  headers = {
    "Authorization" = "Basic YWRtaW46cmVnaXN0cnkx"
  }
}

resource "foxtools_file_download" "chmod" {
  url      = "http://localhost/file.json"
  filename = "/tmp/file.json"
  headers = {
    "Authorization" = "Basic YWRtaW46cmVnaXN0cnkx"
  }
  file_chmod = "444"
}