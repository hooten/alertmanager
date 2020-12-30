# TLS Config Example

## Usage
1. Add [`certs/ca.pem`](certs/ca.pem) to your trusted roots.
2. Install dependencies:
   1. `go install github.com/cloudflare/cfssl/cmd/cfssl`
   2. `go install github.com/cloudflare/cfssl/cmd/cfssl`
   3. `go install github.com/mattn/goreman`
3. `make start` (inside this directory)

## Testing
1. Navigate to one of the Alertmanager instances at `localhost:9093`.
2. Create a silence.
3. Navigate to the other Alertmanager instance at `localhost:9094`.
4. Observe that the silence created in the other Alertmanager instance has been synchronized over to this instance.

