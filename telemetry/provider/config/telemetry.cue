// Schema for hex/telemetry configuration.

service_name?:    string
service_version?: string
environment?:     string

exporter?: "" | "stdout" | "otlp" | "grpc" | "otlp-grpc" | "none" | "off" | "disabled"
