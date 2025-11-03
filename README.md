# RabbitMQ Control Plane Proxy

A high-performance TCP gateway for RabbitMQ that handles SSL termination, authentication, and protocol validation while acting as a transparent proxy between clients and RabbitMQ servers.

## Documentation Reference
- https://www.rabbitmq.com/resources/specs/amqp0-9.pdf

## Features

- SSL/TLS termination
- Authentication and authorization
- Full AMQP 0-9-1 protocol support
- High-performance TCP relay
- Connection monitoring and metrics
- Protection against malformed AMQP frames
## Architecture


sequenceDiagram
    participant C as Client
    participant P as Proxy
    participant R as RabbitMQ

## Phase 1:
    %% 1. Client to Proxy Connection
    First Connection C,P: 1. Client to Proxy Connection
    C->>P: TCP SYN
    P->>C: TCP SYN-ACK
    C->>P: TCP ACK
    
    %% 2. Client TLS Handshake (Proxy as Server)
    C->>P: ClientHello
    P->>C: ServerHello, Certificate, ...
    C->>P: Key Exchange, Change Cipher Spec
    P->>C: Change Cipher Spec, Finished
    
    %% 3. Client AMQP Handshake (Proxy as Server)
    Note over C,P: 3. Client AMQP (Proxy as Server)
    C->>P: AMQP Header (8 bytes)
    P-->>C: Connection.Start (from proxy)
    C->>P: Connection.Start-Ok (with credentials)
    P-->>C: Connection.Tune (from proxy)
    C->>P: Connection.Tune-Ok
    C->>P: Connection.Open (to proxy vhost)
    P-->>C: Connection.Open-Ok (from proxy)
    
    %% 4. Proxy to RabbitMQ Connection (Only reach here if authentication is successful in phase 1)
    Note over P,R: 4. Proxy to RabbitMQ Connection
    P->>R: TCP SYN
    R->>P: TCP SYN-ACK
    P->>R: TCP ACK

## Phase 2:
    %% 6. RabbitMQ AMQP Handshake (Proxy as Client)
    Note over P,R: 6. RabbitMQ AMQP (Proxy as Client)
    P->>R: AMQP Header (8 bytes)
    R-->>P: Connection.Start
    P->>R: Connection.Start-Ok (with proxy credentials)
    R-->>P: Connection.Tune
    P->>R: Connection.Tune-Ok
    P->>R: Connection.Open (to target vhost)
    R-->>P: Connection.Open-Ok
    
## Phase 3:
    %% 7. Connection Handover  (TCP Relay)
    Note over C,R: 7. Connection Handover (TCP Relay)
    C<<->>R: TCP Relay Connection
    
## Phase 4:
    %% 8. Data Flow (Bidirectional)
    Note over C,R: 8. Data Flow (Bidirectional)
    loop AMQP Frames
        C<<->>R: Data Flow
    end

## License

MIT