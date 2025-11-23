# Fuzz Testing Guide

This document describes the fuzz testing implementation for the WebDAV filesystem.

## Overview

Fuzz testing helps discover security vulnerabilities and edge cases by automatically generating random test inputs. The WebDAV filesystem implementation includes comprehensive fuzz tests to ensure robustness against malformed or malicious input.

## Implemented Fuzz Tests

### 1. FuzzXMLParsing

Tests XML multistatus parsing with malformed input.

**Attack Vectors Tested:**
- Billion laughs (entity expansion attacks)
- XXE (external entity injection)
- Invalid UTF-8 encoding
- Unclosed tags and malformed XML structure
- Very deep nesting (stack overflow attempts)
- Huge documents (memory exhaustion)

**Run:**
```bash
go test -fuzz=FuzzXMLParsing -fuzztime=1m
```

### 2. FuzzPathEncoding

Tests path encoding/decoding with various edge cases.

**Attack Vectors Tested:**
- Null bytes (`/path\x00hidden`)
- Unicode normalization (different representations of same path)
- Very long paths (>4096 characters)
- Special characters (control characters, emojis)
- URL encoding tricks (double encoding, mixed encoding)

**Run:**
```bash
go test -fuzz=FuzzPathEncoding -fuzztime=1m
```

### 3. FuzzHTTPResponse

Tests HTTP response parsing with malformed data.

**Attack Vectors Tested:**
- Invalid Content-Length (negative, very large, mismatched)
- Truncated responses (incomplete XML, headers)
- Invalid status codes (non-numeric, out of range)
- Header injection (\r\n in header values)

**Run:**
```bash
go test -fuzz=FuzzHTTPResponse -fuzztime=1m
```

### 4. FuzzAuthHeaders

Tests authentication header parsing.

**Attack Vectors Tested:**
- Malformed Basic authentication
- Invalid Bearer tokens
- Header injection attempts
- Empty or truncated auth values

**Run:**
```bash
go test -fuzz=FuzzAuthHeaders -fuzztime=1m
```

### 5. FuzzPropertyValues

Tests parsing of WebDAV property values.

**Attack Vectors Tested:**
- Invalid integers (negative, overflow)
- Malformed timestamps
- Unusual MIME types
- Empty and invalid property values

**Run:**
```bash
go test -fuzz=FuzzPropertyValues -fuzztime=1m
```

## Running Fuzz Tests

### Quick Test (All Tests)

Run all fuzz tests for 30 seconds each:

```bash
go test -fuzz=FuzzXMLParsing -fuzztime=30s
go test -fuzz=FuzzPathEncoding -fuzztime=30s
go test -fuzz=FuzzHTTPResponse -fuzztime=30s
go test -fuzz=FuzzAuthHeaders -fuzztime=30s
go test -fuzz=FuzzPropertyValues -fuzztime=30s
```

### Continuous Fuzzing

Run until a failure is found:

```bash
go test -fuzz=FuzzXMLParsing
```

Press `Ctrl+C` to stop.

### Using Seed Corpus

Go automatically saves interesting inputs in `testdata/fuzz/` directories. To view the corpus:

```bash
ls testdata/fuzz/FuzzXMLParsing/
```

To add custom seed inputs, create files in these directories with hex-encoded input.

## CI/CD Integration

Fuzz tests run automatically in GitHub Actions CI for 30 seconds each on:
- Ubuntu with Go 1.23
- Every push and pull request

See `.github/workflows/ci.yml` for details.

## Best Practices

### Local Development

1. **Before committing:** Run fuzz tests for at least 1 minute each
   ```bash
   for test in FuzzXMLParsing FuzzPathEncoding FuzzHTTPResponse FuzzAuthHeaders FuzzPropertyValues; do
     echo "Running $test..."
     go test -fuzz=$test -fuzztime=1m
   done
   ```

2. **Weekly:** Run extended fuzzing (10+ minutes per test) to discover rare edge cases

3. **After changes:** If you modify XML parsing, HTTP handling, or path processing, run the relevant fuzz test for at least 5 minutes

### Analyzing Failures

If a fuzz test finds a failure:

1. The failing input is saved to `testdata/fuzz/<TestName>/`
2. Re-run the test to reproduce:
   ```bash
   go test -v
   ```
3. The failing input will be tested as part of the seed corpus
4. Fix the code to handle the input gracefully
5. Verify the fix:
   ```bash
   go test -fuzz=<TestName> -fuzztime=1m
   ```

## Real-World Test Cases

Consider adding corpus entries from production WebDAV servers:

- **Nextcloud** - Popular personal cloud storage
- **ownCloud** - Enterprise file sync and share
- **Apache mod_dav** - Classic WebDAV server
- **nginx WebDAV module** - High-performance option

To capture real responses:

```bash
# Capture PROPFIND response
curl -X PROPFIND -H "Depth: 1" https://your-webdav-server.com/path/ > response.xml

# Add to corpus (as hex)
xxd -p response.xml > testdata/fuzz/FuzzXMLParsing/real-nextcloud-response
```

## Security Benefits

Fuzz testing helps protect against:

1. **XML Injection** - Malicious XML in server responses
2. **Path Traversal** - Directory traversal via crafted paths
3. **Header Injection** - HTTP header injection attacks
4. **DoS** - Memory exhaustion and infinite loops
5. **Data Corruption** - Handling of truncated/corrupted network data
6. **Authentication Bypass** - Malformed auth headers

## Performance Notes

- Fuzz tests use multiple CPU cores (default: number of cores)
- Limit workers with `-parallel` flag:
  ```bash
  go test -fuzz=FuzzXMLParsing -parallel=4
  ```
- Monitor memory usage during extended runs
- Corpus size grows over time; clean up periodically:
  ```bash
  rm -rf testdata/fuzz/*/
  ```

## References

- [Go Fuzzing Tutorial](https://go.dev/doc/tutorial/fuzz)
- [WebDAV RFC 4918](https://tools.ietf.org/html/rfc4918)
- [XML Security Cheatsheet](https://cheatsheetseries.owasp.org/cheatsheets/XML_Security_Cheat_Sheet.html)
- [URL Encoding Best Practices](https://www.w3.org/TR/url-1/)
- [OWASP Testing Guide](https://owasp.org/www-project-web-security-testing-guide/)

## Contributing

When adding new functionality:

1. Add relevant fuzz test seeds
2. Run fuzz tests for at least 5 minutes
3. Document any new attack vectors
4. Update this guide if adding new fuzz tests

## License

Same as the main project (see LICENSE file).
