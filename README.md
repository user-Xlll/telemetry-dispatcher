# telemetry-dispatcher

`telemetry-dispatcher` is a small Go process launcher packaged with an OpenTelemetry Collector distribution. It waits for configured files to appear, starts a child executable, and forwards the child process's standard output and error streams. In the Kubernetes deployment, the bundled collector accepts OTLP telemetry, enriches and batches log records, and exports logs to OpenSearch.

## Purpose

The dispatcher provides a lightweight startup gate for workloads whose configuration or credentials are mounted asynchronously. It can wait for one or more files before starting the target binary, avoiding an immediate startup failure when those files are not yet present. The same container image also includes the collector distribution used by the Helm deployment.

## Functionality

- Runs a child executable with its arguments and returns an error if the child cannot start or exits unsuccessfully.
- Streams child stdout and stderr to the corresponding dispatcher output streams.
- Optionally waits for multiple files before launching the child.
- Provides configurable retry count and delay for file checks.
- Runs a bundled OpenTelemetry Collector in the deployed container; the chart's sample configuration receives OTLP logs, adds deployment attributes and a log index prefix, batches records, and exports them to OpenSearch. Metrics and traces are accepted but configured with a no-op exporter in the chart defaults.
- Includes Kubernetes/Helm resources for the application and Istio ingress/egress gateways. The sample chart exposes the collector health-check port and OTLP gRPC/HTTP endpoints.

## CLI usage

```text
telemetry-dispatcher exec [--wait-file PATH ...] [--wait-retries N] [--wait-interval DURATION] -- BINARY [ARG ...]
```

Examples:

```sh
# Start a command directly
telemetry-dispatcher exec -- /usr/local/bin/my-service --config /etc/my-service/config.yaml

# Wait for both mounted credential files before starting the service
telemetry-dispatcher exec \
  --wait-file /mnt/pvs/username \
  --wait-file /mnt/pvs/password \
  --wait-retries 10 \
  --wait-interval 2s \
  -- /usr/local/bin/my-service
```

The `--` separator is optional when the child arguments do not need to be separated from dispatcher flags. `--wait-retries` defaults to `3`; `--wait-interval` defaults to `1s`. With no `--wait-file` options, the dispatcher proceeds immediately. A missing file after all configured checks causes startup to fail. File paths are checked for existence; the dispatcher does not validate their contents.

## Build and test

The project uses Go 1.26.1 and vendors its Go dependencies.

```sh
go test ./...
go build -o telemetry-dispatcher .
```

The Docker build also downloads a versioned OpenTelemetry Collector distribution from any registry and verifies it against the downloaded SHA-256 checksum. Building the container therefore requires access to that repository and its build credentials:

```sh
docker build \
  --build-arg NEXUS_USERNAME="$NEXUS_USERNAME" \
  --build-arg NEXUS_PASSWORD="$NEXUS_PASSWORD" \
  --tag telemetry-dispatcher:local \
  .
```

Do not commit credentials or pass them through public build logs. For local execution, provide the collector configuration and any required credential files through your own runtime setup.

## Deployment

The repository includes a Helm chart under `deployment/helm` and a Jenkins pipeline in `Jenkinsfile`. The pipeline builds and pushes the OCI image, records its immutable digest, packages and publishes the Helm chart, and runs AppSec and SonarQube checks. Release artifacts are published configured registries; CI requires access to its Jenkins shared libraries, registries, Nexus, and security services.

The chart values in `deployment/helm/values.yaml` are templates/defaults, not a ready-to-deploy environment configuration. Before deployment, supply the environment-specific image, project and gateway hostnames, collector exporter endpoints, secret-manager paths/namespaces, and certificate settings. The chart expects OpenSearch credentials to be mounted at `/mnt/pvs/username` and `/mnt/pvs/password`; configure secret delivery and network egress for the target environment. Its default OTLP receiver listens on ports `4317` (gRPC) and `4318` (HTTP), and the health check listens on `13133`.

Use your CI release and Helm deployment process for a target environment; this repository does not define a public deployment URL or standalone production release command.

## Configuration notes

- The collector configuration is supplied to the container through the chart-mounted ConfigMap, at `/mnt/telemetry-dispatcher/config.yaml`.
- The chart's sample log processing inserts `segment`, `datacenter`, and `stand` attributes and uses them with the service name to form a log index prefix.
- Review the chart's debug exporter and sample endpoints before production use; customize exporters, security, resource limits, ingress, and egress for the destination environment.
- The image's Docker build downloads the collector distribution from an internal repository, so a public-only build environment cannot complete that stage without the required network access and credentials.
