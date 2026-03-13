# Govm Sandbox Example

This example is a standalone, offline demo of the `govm` sandbox backend. It shows three behaviors in one run:

- readonly mount can be read but not written
- readwrite shared mount writes files back to the host
- session workspace is auto-created under `workspace/<session-id>` and mounted at `/workspace`

## Run

From the repository root:

```bash
go run ./examples/13-govm-sandbox
```

## Expected output

The example prints five step results:

- `STEP 1 READONLY_READ: OK`
- `STEP 2 READONLY_WRITE: EXPECTED_DENIED`
- `STEP 3 SHARED_WRITE: OK`
- `STEP 4 WORKSPACE_WRITE: OK`
- `STEP 5 HOST_VERIFY: OK`

## Host files produced

After a successful run, these host-side files should exist:

- `examples/13-govm-sandbox/testdata/shared/result.txt`
- `examples/13-govm-sandbox/workspace/govm-example-session/session-note.txt`
