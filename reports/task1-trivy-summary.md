# Task 1: Docker Image Vulnerability Scan

## Goal

The Docker image was scanned with Trivy before and after optimizing the runtime image. The optimization removes vulnerable Alpine runtime packages from the final image and produces a Trivy JSON report for the CI artifact.

## Before optimization

Image: `product-catalog:latest`

Runtime base image before optimization:

```dockerfile
FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=builder /api-server .

ENTRYPOINT ["./api-server"]
```

Trivy result before optimization:

| Severity | Count |
| --- | ---: |
| CRITICAL | 0 |
| HIGH | 2 |
| MEDIUM | 5 |
| LOW | 3 |

Total: 10 vulnerabilities

The vulnerabilities were detected in `product-catalog:latest (alpine 3.19.9)`. The Go binary target `app/api-server` had 0 detected vulnerabilities. The findings came from Alpine runtime packages such as `busybox`, `musl`, `musl-utils`, `busybox-binsh`, and `ssl_client`.

Before report:

- `reports/trivy-before.json`

## Optimization

The final runtime stage was changed from `alpine:3.19` to `scratch`.

Current optimized runtime stage:

```dockerfile
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /api-server /api-server

EXPOSE 8080

USER 65532:65532

ENTRYPOINT ["/api-server"]
```

What was optimized:

- Removed the Alpine runtime base image from the final image.
- Removed runtime packages such as `busybox`, `musl`, and `ssl_client` from the final image.
- Kept only the statically linked Go binary in the runtime image.
- Copied CA certificates from the builder stage so HTTPS/TLS certificate validation still works if needed.
- Added `USER 65532:65532` so the container does not run as root.
- Added `-ldflags="-s -w"` to reduce the Go binary size by stripping debug and symbol information.

The builder stage still uses `golang:1.26-alpine`, but the builder image is not part of the final runtime image.

## After optimization

Image: `product-catalog:latest`

Runtime base image after optimization: `scratch`

Trivy result after optimization:

| Severity | Count |
| --- | ---: |
| CRITICAL | 0 |
| HIGH | 0 |
| MEDIUM | 0 |
| LOW | 0 |

Total: 0 vulnerabilities

Optimized reports:

- `reports/trivy-optimized.txt`
- `reports/trivy-report.json`

`reports/trivy-report.json` is the local JSON artifact generated with:

```powershell
trivy image --format json --output reports\trivy-report.json product-catalog:latest
```

## CI JSON artifact

The GitHub Actions workflow contains a `trivy-scan` job. It generates a JSON report and uploads it as an artifact:

```yaml
- name: Generate Trivy JSON report
  uses: aquasecurity/trivy-action@master
  with:
    image-ref: product-catalog:${{ github.sha }}
    format: json
    output: trivy-report.json
    exit-code: "0"
    severity: CRITICAL,HIGH,MEDIUM,LOW

- name: Upload Trivy JSON report
  if: always()
  uses: actions/upload-artifact@v4
  with:
    name: trivy-report
    path: trivy-report.json
```

The workflow also runs a table scan that fails the pipeline if `CRITICAL` or `HIGH` vulnerabilities are found:

```yaml
- name: Scan Docker image with Trivy
  uses: aquasecurity/trivy-action@master
  with:
    image-ref: product-catalog:${{ github.sha }}
    format: table
    exit-code: "1"
    severity: CRITICAL,HIGH
```

## Screenshot commands

Show the optimized scan summary:

```powershell
Get-Content .\reports\trivy-optimized.txt
```

Show the JSON artifact file exists:

```powershell
Get-Item .\reports\trivy-report.json | Format-List FullName,Length,LastWriteTime
```

Show the Dockerfile optimization:

```powershell
git diff -- Dockerfile
```

Show the Trivy CI artifact configuration:

```powershell
git diff -- .github\workflows\ci.yml
```
