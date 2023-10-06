# TCP Load Balancer with mTLS

This repository provides an implementation of a TCP load balancer. The load balancer distributes incoming client requests across a collection of configured backend servers. This load balancer utilizes mutual TLS (mTLS) for security purposes and has client-based rate limiting capabilities.

## Features:
- **mTLS Support**: Supports TLS for encrypted connections and mutual TLS for client-server authentication. This ensures a two-way verification process, offering a higher level of security compared to traditional TLS.
- **Client Authentication and Authorization**: Authenticates clients based on their TLS certificates and authorizes them based on an access control list.
- **Rate Limiter**: Restricts the number of requests a particular client can make.
- **Backend Server Selection**: Chooses a backend server based on least connections.
- **Graceful Shutdown**: Ensures that the server started or stopped gracefully, and ongoing connections are not abruptly terminated.
- **Configuration Management**: Easily configurable using a JSON configuration file.

## Prerequisites
- Go v1.21.1
- Valid TLS v1.3 certificates for client-server mTLS authentication. Please follow the [steps to generate the cerficates](/doc/mTLS-guide.md), if needed.


## Installation
1. Clone the repository:
   ```bash
   git clone https://github.com/rrasulzade/tcp-lb-go.git
   ```

1. Navigate to the project directory:
   ```bash
   cd tcp-lb-go
   ```

1. Build the project:
   ```bash
   go build
   ```

## Running the Server

To run the server, use:
```bash
   ./tcp-lb-go -config /path/to/config.json
```

To view the available flags and their descriptions, use:
```bash
  ./tcp-lb-go -h
```

## Configuration

The load balancer is configured using a JSON configuration file. The configuration includes settings for the server port, backend servers, TLS configurations, rate limiter settings, allowed clients, and client-backend access control lists.

Sample configuration:

```json
{
  "port": 3003,
  "backends": ["backend1:port", "backend2:port"],
  "tls": {
    "cert_file": "/path/to/cert.pem",
    "key_file": "/path/to/key.pem",
    "ca_file": "/path/to/ca.pem"
  },
  "rate_limiter": {
    "capacity": 10,
    "refill_rate": 2
  },
  "allowed_clients": {
    "client1.example.com": true,
    "client2.exapmle.com": true
  },
  "client_backend_acl": {
    "a3f1c63a8f01b4f4e061c10d7b4b1a7e2d4e223b...": [
      "backend1"
    ],
    "b2d2e4423c10d7b4b1a7e2d4e223ba4f5e061c1d...": [
      "backend1",
      "backend2"
    ]
  }
}
```

#### `port`
- **Description**: The port number on which the load balancer server runs.

#### `backends`
- **Description**: List of backend servers' addresses to which the load balancer will distribute incoming TCP connections.

#### `tls`
- **Description**: Contains the TLS configuration settings for encrypted connections.
  - `cert_file`: Path to the server's certificate file.
  - `key_file`: Path to the server's private key file.
  - `ca_file`: Path to the root Certificate Authority (CA) file used to verify client certificates for mutual TLS authentication.

#### `rate_limiter`
- **Description**: Contains the rate limiting settings using a token bucket algorithm.
  - `capacity`: Maximum number of tokens in the bucket.
  - `refill_rate`: Number of tokens added to the bucket every second.

#### `allowed_clients`
- **Description**: A map of client Common Names (CN) that are allowed to connect. The CN is extracted from the client's TLS certificate. If the CN is present and set to `true`, the client is allowed.

#### `client_backend_acl`
- **Description**: Defines the access control list for clients and backends. It maps a client's ID to the backends it's allowed to access. If a backend is listed for a client, the client can access it. 
- **Client ID Format**: The clientID is generated by hashing the client's `CommonName` and `SerialNumber` combined with `:` separator in between from the TLS certificate using the SHA-256 algorithm. The resulting hash is then converted to a hexadecimal string. This ensures a unique ID for each client based on their certificate details.

## Testing the Load Balancer

Before testing the load balancer, need to set up some backend servers. One of the easiest ways to do this is by using the `http-server` package, which serves static files over HTTP.

### Setting up Backend Servers

1. Install `http-server` globally:
   ```bash
   npm install -g http-server
   ```

1. Navigate to the directory containing the files you want to serve. If you don't have specific files, you can create a simple `index.html` for testing purposes. 
<br>

1. Start a backend server on any port, (e.g.,`5010`), using the following command:
   ```bash
   npx http-server -p 5010
   ```

You can start multiple backend servers by changing the port number in the command above.

### **Using curl with TLS**
Once the load balancer and backend servers up and running, you can simulate client requests.

**`curl`** is a command-line tool that can be used to send requests. If your load balancer is set up with TLS, you can use curl to send secure requests.

Use the following command to send a request to the load balancer:

```bash
curl --cert path/to/client.crt --key path/to/client.key  --cacert path/to/rootCA.pem https://localhost:<LOAD_BALANCER_PORT>
```
Replace `<LOAD_BALANCER_PORT>` with the port number on which the load balancer is running.