# githubowners-interceptor

Install pipeline and triggers before installing the github-owners cluster interceptor

## Install github-owners-interceptors

``` bash
ko apply --sbom=none --base-import-paths -f config
```

## Local Testing

For local testing reference to the `examples` folder