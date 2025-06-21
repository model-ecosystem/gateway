# gRPC Transcoding Example

This example demonstrates how to use the gateway's gRPC transcoding feature to convert HTTP/JSON requests to gRPC/protobuf.

## Overview

The gateway supports automatic transcoding between HTTP/JSON and gRPC/protobuf protocols, allowing REST clients to communicate with gRPC services.

## Configuration

To enable gRPC transcoding for a route, add the `grpc` configuration section:

```yaml
gateway:
  router:
    rules:
      - id: grpc-users
        path: /api/users/*
        serviceName: user-service
        loadBalance: round_robin
        grpc:
          enableTranscoding: true
          service: "users.v1.UserService"
          protoDescriptorBase64: "CgpwYXNzZW5nZXIucHJvdG8..."
```

## Proto Descriptor Generation

1. Create your proto file (e.g., `user.proto`):
```proto
syntax = "proto3";
package users.v1;

message GetUserRequest {
  string id = 1;
}

message User {
  string id = 1;
  string name = 2;
  string email = 3;
}

service UserService {
  rpc GetUser(GetUserRequest) returns (User);
}
```

2. Generate the descriptor set:
```bash
protoc --descriptor_set_out=user.pb \
       --include_imports \
       --include_source_info \
       user.proto
```

3. Convert to base64:
```bash
base64 user.pb > user.pb.b64
```

4. Copy the base64 content to your configuration.

## HTTP to gRPC Mapping

The gateway automatically maps HTTP requests to gRPC methods:

- HTTP Path: `/api/users/123`
- gRPC Method: `/users.v1.UserService/GetUser`
- Request Body: `{"id": "123"}`

## Features

- **Automatic Protocol Detection**: Routes with gRPC configuration automatically use the gRPC connector
- **JSON/Protobuf Transcoding**: Seamless conversion between formats
- **Proto Descriptor Support**: Load from file or base64-encoded string
- **Fallback Mode**: If transcoding fails, passes through JSON (useful for development)

## Testing

1. Start a gRPC service on port 9090
2. Configure the gateway with the example configuration
3. Send HTTP requests:

```bash
# Get a user
curl -X POST http://localhost:8080/api/users/GetUser \
  -H "Content-Type: application/json" \
  -d '{"id": "123"}'

# The gateway will:
# 1. Convert JSON to protobuf
# 2. Call the gRPC service
# 3. Convert the protobuf response back to JSON
```

## Advanced Configuration

### Custom Transcoding Rules

You can define custom HTTP-to-gRPC mappings:

```yaml
grpc:
  enableTranscoding: true
  service: "users.v1.UserService"
  protoDescriptorBase64: "..."
  transcodingRules:
    "GET /users/{id}": "GetUser"
    "POST /users": "CreateUser"
    "PUT /users/{id}": "UpdateUser"
    "DELETE /users/{id}": "DeleteUser"
```

### Multiple Proto Files

For services with multiple proto files, use `protoc` with `--include_imports`:

```bash
protoc --descriptor_set_out=service.pb \
       --include_imports \
       --include_source_info \
       *.proto
```

## Limitations

- Streaming RPCs are not yet supported
- Proto3 JSON mapping follows Google's JSON mapping specification
- Binary fields are base64-encoded in JSON

## Troubleshooting

1. **"Failed to load proto descriptors"**: Check that the base64 string is valid
2. **"Transcoding failed, using pass-through"**: Verify the proto descriptor matches your service
3. **"Method not found"**: Ensure the gRPC path format is correct: `/package.Service/Method`