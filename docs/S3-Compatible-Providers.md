# S3-Compatible Providers

File Mover Express (FME) works with Amazon S3 and with any storage service that implements the S3 API. A Remote
Configuration selects a **storage provider**; the daemon then applies the behaviour that provider needs (endpoint,
addressing style, request checksums, storage classes, deletion strategy) and the GUI prefills sensible defaults.

Built-in presets:

| Provider | `provider` value | Endpoint URL pattern | Region | Path-style | Notes |
|---|---|---|---|---|---|
| Amazon S3 | `aws` (or omitted) | none (optional Access Point / VPC endpoint) | the bucket's AWS Region | off | Full feature set: OIDC / SSO, Transfer Acceleration, AWS storage classes |
| MinIO | `minio` | `https://minio.example.com:9000` (API port, not the console) | `us-east-1` (any value works) | **on** | Versioning optional; FME falls back to key-based deletes when it is off |
| Cloudflare R2 | `cloudflare-r2` | `https://<account-id>.r2.cloudflarestorage.com` | `auto` | off | No object versioning API; only the `standard` storage class |
| Backblaze B2 | `backblaze-b2` | `https://s3.<region>.backblazeb2.com` | must match the endpoint, e.g. `us-west-004` | off | Use an *application key* (key ID + key) |
| Wasabi | `wasabi` | `https://s3.<region>.wasabisys.com` | must match the endpoint, e.g. `us-east-1` | off | Storage classes are not sent |
| DigitalOcean Spaces | `digitalocean-spaces` | `https://<region>.digitaloceanspaces.com` | `us-east-1` (DigitalOcean's recommendation for S3 clients) | off | No object versioning API |
| Hetzner Object Storage | `hetzner` | `https://<location>.your-objectstorage.com` | the bucket's location code: `fsn1`, `nbg1` or `hel1` | off | Versioning supported; storage classes are not sent; CopyObject only within a bucket |
| Other S3-compatible | `other` | full URL of the S3 API | `us-east-1` unless documented otherwise | **on** | Ceph RGW, Garage, SeaweedFS, Storj, NetApp, Dell ECS, ... |

> The setup wizard shown on first launch is for Amazon S3. To use another provider, open the **Remote Configuration**
> dropdown and choose **Add**.

## Setting up a provider in the GUI

1. In the S3 Bucket browser, open the Remote Configuration dropdown and choose **Add Remote Configuration**.
2. On the **Connection** tab:
   - Choose the **Storage provider**. The endpoint URL pattern, region default and addressing style are prefilled.
   - Enter a **Remote Configuration name** and the **S3 bucket**.
   - Complete the **Endpoint URL** (replace placeholders such as `<account-id>` or `<region>`) and check the **Region**.
   - Leave **Use path-style addressing** as prefilled unless your provider's documentation says otherwise.
   - If the endpoint uses a private or self-signed certificate, set a **Custom CA bundle** (preferred) or, for lab
     networks only, **Skip TLS certificate verification**.
3. On the **Authentication** tab choose **Access keys** and enter the key ID and secret your provider issued.
   A named profile from `~/.aws/credentials` on the daemon machine also works for any provider. OIDC / SSO is only
   available for Amazon S3 because it relies on AWS STS.
4. Back on the **Connection** tab, click **Test connection**. FME connects with the values in the form (nothing is
   saved yet) and lists the bucket once. Fix anything the message points at, then choose **Add**.

Transfer Acceleration and the AWS storage class list are hidden for non-Amazon providers.

## Configuration file

Everything the GUI does can be written directly to `~/.filemoverexpress/configuration.yaml`. Keys are camelCase;
`authMethod` is numeric: `1` named profile, `2` OIDC / SSO (Amazon S3 only), `3` access keys.

### MinIO (self-hosted, http, path-style)

```yaml
protocols:
  s3:
    transferProfiles:
      minio-local:
        name: "minio-local"
        provider: minio
        endpoint: "http://192.168.1.20:9000"
        pathStyle: true
        region: "us-east-1"
        bucket: "media"
        authMethod: 3
        staticCredentials:
          accessKeyId: "minioadmin"
          secretAccessKey: "minioadmin"
        autoTuning: true
```

### Cloudflare R2

```yaml
      r2-archive:
        name: "r2-archive"
        provider: cloudflare-r2
        endpoint: "https://0123456789abcdef0123456789abcdef.r2.cloudflarestorage.com"
        region: "auto"
        bucket: "camera-archive"
        authMethod: 3
        staticCredentials:
          accessKeyId: "<R2 access key id>"
          secretAccessKey: "<R2 secret access key>"
```

Create the keys under *R2 → Manage R2 API Tokens*; the token needs *Object Read & Write* on the bucket.

### Backblaze B2

```yaml
      b2-dailies:
        name: "b2-dailies"
        provider: backblaze-b2
        endpoint: "https://s3.us-west-004.backblazeb2.com"
        region: "us-west-004"
        bucket: "dailies"
        authMethod: 3
        staticCredentials:
          accessKeyId: "<application keyID>"
          secretAccessKey: "<application key>"
```

The endpoint and region are shown on the bucket's details page. Use an application key scoped to the bucket.

### Wasabi

```yaml
      wasabi-ingest:
        name: "wasabi-ingest"
        provider: wasabi
        endpoint: "https://s3.eu-central-1.wasabisys.com"
        region: "eu-central-1"
        bucket: "ingest"
        authMethod: 3
        staticCredentials:
          accessKeyId: "<access key>"
          secretAccessKey: "<secret key>"
```

### DigitalOcean Spaces

```yaml
      spaces-fra1:
        name: "spaces-fra1"
        provider: digitalocean-spaces
        endpoint: "https://fra1.digitaloceanspaces.com"
        region: "us-east-1"
        bucket: "my-space"
        authMethod: 3
        staticCredentials:
          accessKeyId: "<Spaces access key>"
          secretAccessKey: "<Spaces secret>"
```

### Hetzner Object Storage

```yaml
      hetzner-fsn1:
        name: "hetzner-fsn1"
        provider: hetzner
        endpoint: "https://fsn1.your-objectstorage.com"
        region: "fsn1"
        bucket: "rushes"
        authMethod: 3
        staticCredentials:
          accessKeyId: "<S3 access key>"
          secretAccessKey: "<S3 secret key>"
```

Create the key pair under the bucket's **S3 Keys** section in the Hetzner Cloud Console. The region must be the
bucket's location code (`fsn1` Falkenstein, `nbg1` Nuremberg, `hel1` Helsinki) and the endpoint must use the same code.

### Any other S3-compatible store

```yaml
      ceph-rgw:
        name: "ceph-rgw"
        provider: other
        endpoint: "https://rgw.studio.internal"
        pathStyle: true
        region: "us-east-1"
        bucket: "rushes"
        authMethod: 1               # named profile from ~/.aws/credentials on the daemon machine
        profile: "ceph"
        tls:
          caBundle: "/etc/ssl/studio-ca.pem"
```

### Field reference

| Key | Type | Description |
|---|---|---|
| `provider` | string | One of the values in the table above. Omitted or empty means Amazon S3. Unknown values are treated as `other` and produce a warning. |
| `endpoint` | string | Full URL of the S3 API. Required for every provider except Amazon S3. |
| `pathStyle` | bool | `true` sends requests as `endpoint/bucket/key`; `false` uses `bucket.endpoint/key`. Ignored for Amazon S3. |
| `region` | string | Signing region. Defaults per provider when omitted (see table). |
| `authMethod` | int | `1` named profile (default), `2` OIDC / SSO (Amazon S3 only), `3` access keys. |
| `staticCredentials.accessKeyId` | string | Access key ID (required with `authMethod: 3`). |
| `staticCredentials.secretAccessKey` | string | Secret access key (required with `authMethod: 3`). |
| `staticCredentials.sessionToken` | string | Optional session token for temporary credentials. FME does not refresh it. |
| `tls.caBundle` | string | PEM file on the daemon machine appended to the system trust store for this endpoint. |
| `tls.skipVerify` | bool | Disables certificate verification. Insecure; the daemon logs a warning once per Remote Configuration each time it starts. |

All other keys (`storageClass`, `checksums`, `chunkSize`, `threads`, `filter`, `maxAge`, `fileOrder`, `paths`, ...)
work exactly as described in [Configuration](Configuration.md).

### Where the access keys live

With `authMethod: 3` the keys are stored in plain text in `configuration.yaml`, which FME saves with owner-only
permissions on macOS and Linux, the same posture as `~/.aws/credentials`. They are redacted from support files
(`filemoverexpress support-file`) and never written to the logs. The daemon never sends the secret access key or
session token to the GUI: the form shows them as saved, and they are kept unless you type new ones. If you change
the access key ID, or re-create the Remote Configuration under another name, enter the secret again.

If you prefer not to store keys in FME's configuration, keep `authMethod: 1` and either create a profile in
`~/.aws/credentials` on the daemon machine or export `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` in the daemon's
environment; both work for any provider.

## What FME does differently for non-Amazon providers

- **Request checksums**: the AWS SDK adds CRC32 checksum trailers to uploads by default; several S3-compatible
  services reject them. FME only sends checksums where the S3 protocol requires them. FME's own end-to-end file
  checksums (MD5, XXHash, XXH3) are unaffected and still stored as object metadata.
- **Storage classes**: the `x-amz-storage-class` header is omitted for providers that do not support it, and for
  `standard` on every non-Amazon provider. Other values are passed through as entered.
- **Deletes and renames**: where the provider has no object-versioning API (R2, Spaces) or returns
  `NotImplemented`, FME lists and deletes current objects by key instead of enumerating versions. If a provider
  rejects batch deletion, objects are deleted one at a time.
- **Sessions**: AWS STS is not contacted (no `GetCallerIdentity` check, no OIDC role assumption).
- **Transfer Acceleration** and the AWS storage class list are not offered.

## Validating from the command line

```bash
filemoverexpress validate-credentials minio-local
```

This builds the S3 client exactly as a transfer would and lists the bucket once, so endpoint, region, addressing
style, TLS settings and credentials are all exercised.

## Troubleshooting

| Symptom | Likely cause | What to try |
|---|---|---|
| `SignatureDoesNotMatch` | Wrong region, or wrong addressing style for this provider | Use the region from the table (for Hetzner, Backblaze and Wasabi it must match the location in the endpoint); toggle **path-style addressing** |
| `no such host` / DNS error containing the bucket name | Virtual-hosted addressing against a store without wildcard DNS | Turn **path-style addressing** on |
| `x509: certificate signed by unknown authority` | Private or self-signed certificate | Add the CA as a **Custom CA bundle**; use skip-verify only on trusted lab networks |
| `InvalidAccessKeyId` / `AccessDenied` | Wrong or under-privileged key | Check the key and that it can list, read and write the bucket |
| `NotImplemented` on delete | Store without object versioning | Handled automatically; if it persists, update FME and report the provider |
| Uploads fail mentioning `x-amz-checksum` or `STREAMING-...-TRAILER` | Provider rejects checksum trailers | Make sure the Remote Configuration's **Storage provider** is not set to Amazon S3 |
| `an endpoint URL is required for this S3 provider` | `endpoint` missing in the configuration file | Add the full `https://` URL of the provider's S3 API |
| `OIDC / SSO authentication is only available for Amazon S3` | `authMethod: 2` on a non-Amazon provider | Switch to access keys (`authMethod: 3`) or a named profile (`1`) |
