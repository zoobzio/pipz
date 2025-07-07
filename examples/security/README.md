# Security Audit Example

This example demonstrates using pipz for security audit pipelines with access control, data redaction, and compliance tracking.

## Overview

The security pipeline shows how to:
- Implement zero-trust access control
- Create audit trails for compliance
- Redact sensitive data based on permissions
- Track regulatory compliance (GDPR/CCPA)
- Handle different access levels (admin vs regular users)

## Structure

```go
type AuditableData struct {
    Data      *User
    UserID    string
    Timestamp time.Time
    Actions   []string  // Audit trail
}

type User struct {
    Name    string
    Email   string
    SSN     string
    IsAdmin bool
}
```

## Security Steps

1. **CheckPermissions**: Validates user authorization (zero-trust)
2. **ValidateDataIntegrity**: Ensures audit data is properly formed
3. **LogAccess**: Records access for audit trail
4. **RedactSensitive**: Masks PII based on user permissions
5. **TrackCompliance**: Records regulatory compliance

## Usage

```go
// Create security pipeline
securityPipeline := CreateSecurityPipeline()

// Process audit request
audit := AuditableData{
    Data:      userData,
    UserID:    "user123",
    Timestamp: time.Now(),
}

result, err := securityPipeline.Process(audit)
if err != nil {
    // Access denied - zero value returned
    log.Printf("Security check failed: %v", err)
}
```

## Redaction Rules

- **SSN**: Always redacted to show only last 4 digits
- **Email**: Redacted for non-admin users (shows first letter + domain)
- **Admin Access**: Minimal redaction (SSN only)

## Running

```bash
# Run the example
go run security.go

# Run tests
go test
```

## Key Features

- **Zero-Trust**: Every access requires explicit authorization
- **Audit Trail**: Complete record of all operations
- **Role-Based Redaction**: Different data visibility for different roles
- **Compliance**: Built-in GDPR/CCPA compliance tracking
- **Data Integrity**: Validates audit data before processing

## Example Output

```
Test 1: Regular user access
✓ Access granted to user123
  Email: j***@example.com
  SSN: XXX-XX-6789
  Audit trail: [permissions verified accessed by user123 at 2025-01-06T... sensitive data redacted GDPR/CCPA compliant access logged]

Test 2: Admin user access
✓ Admin access granted to admin456
  Email: jane.admin@example.com
  SSN: XXX-XX-4321
  Audit trail: [permissions verified accessed by admin456 at 2025-01-06T... admin view - minimal redaction GDPR/CCPA compliant access logged]

Test 3: Unauthorized access attempt
✗ Access denied: unauthorized: no user ID

Test 4: Invalid data integrity
✗ Integrity check failed: no data to audit
```