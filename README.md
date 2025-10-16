# gcp-signed-urls

## Requirements

- gcp project and bucket created

- service account to upload objects:

   - Go to IAM & Admin → [Service Accounts](https://console.cloud.google.com/iam-admin/serviceaccounts)

   - Create a service account (or use an existing one).

   - Roles needed: `Storage Object Admin`(or `Storage Object Creator`)

   - generate json key for service account and save it

   - `$ export GOOGLE_APPLICATION_CREDENTIALS="/path/to/service-account.json"`

## Usage

- `$ GCS_BUCKET=<your-bucket> go run internal/main.go`