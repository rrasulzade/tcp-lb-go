### Design Approach

By breaking down the project into two distinct components - **a reusable library and a server** - we ensure that each component has a clear and well-defined purpose. Modularization allows for easier development, maintenance, and future expansion of the project. The project will be implemented in Go. 

### Library Component

The library will feature an implementation of a per-client based rate limiter using Token Bucket algorithm, and the Least Connections algortihm to efficiently distribute incoming connections to backend servers based on their current load. Here's an overview of the functionalities:

- **Token Bucket Algorithm**:
  - It will utilize a concurrent dictionary to manage tokens per client at a specified rate.
  - Clients will be consuming a token for each allowed connection, and tokens will be replenished at a controlled rate.

- **Least Connections Forwarder**:
  - It will keep a concurrent list of backend servers to track the number of active connections for each backend.
  - It will manage the list to add new backends and choose the one with least connections as needed.

###  Server Component

The server will act as the interface between clients and the reusable library. It will extend the functionality of the library by adding security measures, connection handling and an authorization layer as outlined below:

- **mTLS Authentication**
  - Mutual Transport Layer Security (mTLS) authentication will be leveraged to establish secure communication between clients and the server.
  - During the connection establishment, both the server and client must present their certificates for mutual authentication.

- **Authorization Scheme**:
  - A static map will be defined within the code to associate clients with permitted backend servers. This access control list (ACL) will be checked for each incoming client request to ensure only authorized clients gain access.

- **Connection Handling**
  - Upon connection initiation, the server will initiate an mTLS handshake to verify the client's identity.
  - Subsequently, authorization check will be performed based on the predefined ACLs.
  - Valid connections will be distributed efficiently among the available backend servers leveraging the features from the library.

The diagram below illustrates communication steps between client, load balancer, and backend server:
<img src="/doc/rfd/tcp-lb-diagram.png" alt="tcp-lb-diagram" width="600"/>


## Library Implementation Details

### Least Connections Forwarder

The load balancer will use the "least connections" algorithm. It will track the number of active connections for each backend server and forward incoming requests to the backend with the fewest active connections. 

#### Proposed API

**`func (b *Backend) IncrementConns()`** 
    - Increments the active connection count of the backend server.

**`func (b *Backend) DecrementConns()`**
    - Decrements the active connection count of the backend server.

**`func (b *Backend) GetConns() int`**
    - Returns the active connection count of the backend server.

**`func (lb *LoadBalancer) AddBackend(backend *Backend)`**
    - Adds a backend server to the list of available backends maintaned by the load balancer.

**`func (lb *LoadBalancer) GetBackend(allowedBackends map[string]bool) (*Backend, error)`**
    - Returns the backend server with the least connections by iterating through the server pool and matching with the provided list of allowed backends for the client, considering the connection count of each backend.
    - *Note:* To achieve a more accurate lookup for finding a backend with the least connections, the connection count of all backend servers have to remain consistent. This method will prioritize correctness over performance. Once a working functionality is achieved, the performance will be revisited.

**`Backend`** struct represents a backend server and will contain the following fields which may be modified during the implementation:
- **`Address`**: The hostname or IP address of the backend server.
- **`Connections`**: The current number of active connections to the backend server.


### Rate Limiter - Token Bucket Algorithm

A token bucket algorithm will be used for rate limiting. Each client will have a token bucket, and tokens will be consumed for each connection request. If a bucket is empty, the connection request will be denied.

#### Proposed Functions API

**`func (rl *RateLimiter) AllowConnection(clientID string, bucketCapacity, bucketRefillRate int) bool`** 
    - Checks if a client is allowed to make a connection based on their rate limits. If so, decrements token count and allows the connection.


## Server Implementation Details

### Authentication

The load balancer server will use mTLS for enhanced authentication, utilizing the TLS v1.3 protocol for both server and clients. The clients will only trust servers that produce a valid certificate signed by server Certificate Authority (CA). We assume that every client will have a unique certificate with a distinct X.509 `CommonName` which will be used to uniquely identify clients for authentication and authorization purposes.

The server will also keep a static map to store the list of allowed client certificate CNs. Moreover, after validating whether the client's certificate is signed by a trusted CA, the server can further authenticate the client by checking the certificate's CN against an allowed CN. This approach helps us maintain a strong TLS authentication between clients and the server. 

```go
var clientAuthenticationMap = map[string]bool{
  "client1.example.com": true,
  "client2.example.com": true,
  // ...
}
```

Further guidance will be shared on setting up a local CA and generating certificate-key pairs using `openssl`.


### Authorization

To control access to backend servers, the load balancer server will manage a statically configured authorization scheme. The format of the scheme will be a dictionary that associates unique client IDs to backend servers. Refering to the assumption in the [Authentication](#authentication) section, since every client has a unique certificate, `clientID` will be a `SHA-256` hashed value of a combination of `CommonName` and `SerialNumber` extracted from the client's certificate as shown in the example below:

```go
var clientACLMap = map[string]map[string]bool{
    "898ea8b3a43cb7e55b5316f7fa5578a11cb9615311bae5931c6796213e6b57f2": {
      "backend1": true,
      "backend3": true,
    },
      
    "7e55b5316f7fa55796213e6b578a11cb9618ae5931c67f298ea8b3a43cb5311b": {
      "backend2": true,
      "backend3": true,
    }, 
    // ...
```


#### Proposed Functions API

**`func (s *Server) Start()`**
    - Sets up a secure socket listener, ready to accept incoming connections.

**`func (s *Server) Stop()`**
    - Gracefully shuts down the server, ensuring all connections are closed.

**`func (s *Server) AuthorizeClient(clientID string) (bool, error)`**
    - Checks if a client is authorized to access specific backend servers based on the given clientID.

**`func (s *Server) AuthenticateClient(clientID string) (bool, error)`**
    - Ensures that only allowed clients with valid and trusted TLS certificates can establish a connection with the server. The list of allowed client certificate CNs will be staticaly defined.

**`func (s *Server) HandleConnection(conn net.Conn)`**
    - Handles the main logic listening for incoming connections and forwarding them to the selected backend server based on the least connections algorithm once the client passes authentication, authorization and rate limiting checks.


## Command Line Flags

To facilitate easy configuration of port number and available backend servers, the following flags are integrated into the load balancer implementation. The command line flags will be `-port` and `-backend`, respectively. The `-port` flag determines the port on which the load balancer service listens. The `-backend` flag is used to specify a comma-separated list of server addresses that the load balancer should distribute traffic to. For example: ` -backend "localhost:5001,127.0.0.1:5002" -port 3003`

## Logging and Monitoring

The load balancer server will be equipped with logging and monitoring capabilities to ensure transparency and traceability. Every connection attempt, whether successful or failed, will be logged with details such as client identity, timestamp, target backend server, and reason for denial. This meticulous and granular logging helps in diagnosing issues, understanding traffic patterns, and detecting potential security threats. Moreover, the system will actively monitor performance of backend servers, ensuring optimal load distribution and timely detection of any server outages. Regular reviews of these logs and metrics will enhance security and provide insights for future optimizations.

