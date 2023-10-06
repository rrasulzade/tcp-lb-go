### Generate mTLS certificates using mkcert and OpenSSL

Below is a step-by-step guide to help understand and execute the process of generating mTLS certificates for local development environment.

1. **Installation of mkcert**
   
   mkcert is a simple zero-config tool to make locally trusted development certificates. If it's not installed on your machine, follow the instructions provided in this [link](https://github.com/FiloSottile/mkcert#installation) to set up `mkcert` on your system.
<br>
1. **Create a Local Certificate Authority (CA)**
   
   ```bash
   mkcert -install
   ```
   *Note:* This step establishes a local CA, making it trusted on your system. It should only be done once.
<br>
1. **Draft a Certificate Configuration File**

   Before generating the certificates, it's helpful to have a configuration file to dictate how these certificates should be structured. 

   A sample configuration looks like:
   ```ini
   [ req ]
   default_bits = 2048
   prompt = no
   default_md = sha256
   req_extensions = req_ext
   distinguished_name = dn

   [ dn ]
   CN = server.example.com
   O = Org
   L = NYC
   ST = NY
   C = US
   emailAddress = admin@localhost

   [v3_req]
   subjectAltName = @alt_names

   [ alt_names ]
   DNS.1 = localhost
   DNS.2 = mydomain.com


   [ req_ext ]
   keyUsage = keyEncipherment, dataEncipherment
   extendedKeyUsage = serverAuth, clientAuth
   subjectAltName = @alt_names
   ```

   This configuration should be saved as `my_cert.conf` preferably in a dedicated `cert` directory.
<br>
1. **Generate a Server Certificate**

   Using OpenSSL and the configuration from the previous step:
   ```bash
   openssl req -new -newkey rsa:2048 -nodes -keyout server.key -out server.csr -config ./ca/my_cert.conf
   openssl x509 -req -in server.csr -CA "$(mkcert -CAROOT)/rootCA.pem" -CAkey "$(mkcert -CAROOT)/rootCA-key.pem" -CAcreateserial -out server.crt -days 3650 -extensions req_ext -extfile ./ca/my_cert.conf
   ```

   Here, `server.crt` and `server.key` are the certificate and private key files respectively for your server.
<br>
1. **Generate a Client Certificate**

   Each client requires a unique certificate. For the first client, adjust the `CN` in the configuration file (e.g.,`CN = client1.example.com`) and run the following commands:
   ```bash
   openssl req -new -newkey rsa:2048 -nodes -keyout client1.key -out client1.csr -config ./ca/my_cert.conf
   openssl x509 -req -in client1.csr -CA "$(mkcert -CAROOT)/rootCA.pem" -CAkey "$(mkcert -CAROOT)/rootCA-key.pem" -CAcreateserial -out client1.crt -days 3650 -extensions req_ext -extfile ./ca/my_cert.conf
   ```

   To add more clients, repeat the commands above, adjusting `CN` in the config file and the filename (e.g., `client2`) as needed.
<br>