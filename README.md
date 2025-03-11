# GLOB

File repository without bells and whistles intended for simple testing or quickly making a file available.

This will never have fancy features like automated expiry, cleanup, security, or the like.

If any of those features are needed, this is not the solution.

You should **NEVER** expose this on a public internet facing IP.

## Features

- Upload files via HTTP Body and `POST/PUT /filename.ext`
- Upload files via HTML form element: visit `/index.html`
- Download files `GET /filename.ext`
- Delete fiels `DELETE /filename.ext`
- Serve list of files as:
    - clickable links `Accept: text/html` (Default header value for browsers)
    - JSON `Accept: application/json text/json`
    - newline separated file names `Accept: */*`

## Usage

Example curl commands or just open it in the browser `http://localhost:3000/index.html`

```bash
# Uploading
curl -X POST localhost:3000/filename.ext -d 'hello world!'
curl localhost:3000/ -F file=@filename.ext

# Retrieving
curl localhost:3000/filename.ext

# List files
curl localhost:3000/

# JSON list
curl localhost:3000/ -H 'Accept: application/json,text/json'
```

## Quickstart

```bash
docker run --name glob \
    --detach --rm \
    --env 'GLOB_PATH=/globs' \
    --volume "$PWD/globs:/globs" \
    --publish '3000:3000' \
    ghcr.io/baarsgaard/glob:latest
```
