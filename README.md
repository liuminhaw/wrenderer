# Wrenderer

## Usage

### Render

```bash
curl -H 'x-api-key: YOUR-API-KEY' "https://wrenderer.example.com/render?url=https://www.target.com"
```

### Cache invalidation

Invalidate single url

```bash
curl -X DELETE -H 'x-api-key: YOUR-API-KEY' "https://wrenderer.example.com/render?url=https://www.target.com"
```

Invalidate whole domain

```bash
curl -X DELETE -H 'x-api-key: YOUR-API-KEY' "https://wrenderer.example.com/render?domain=www.target.com"
```

### Sitemap prerender

Read content from the given sitemap url and render each url to create cache beforehand

```bash
curl -i -X PUT -H 'x-api-key: YOUR-API-KEY' -H "Content-Type: application/json" -d '{"sitemapUrl": "https://wrenderer.example.com/sitemap.xml"}' "https://wrenderer.example.com/render/sitemap"
```

**Response**
```json
{
    "message": "Sitemap rendering accepted", 
    "location": "/render/sitemap/xxxxxx-xxxxxx/status"
}
```

This is an async operation, it will return `202` status code with `location` header to check the status of the operation. The status check location url will also be in the response body with `location` key.

#### Status check
To check the status of the sitemap rendering operation, use the request path from the location URL returned by the operation. This URL is provided either in the location header or in the location key of the response body, which will display the current status of the operation.
```bash
curl -i -X GET -H 'x-api-key: YOUR-API-KEY' "https://wrenderer.example.com/render/sitemap/xxxxxx-xxxxxx/status"
```

### List rendered caches (admin only)

> Note: Currently implement in local build type only

```bash
curl -H 'x-api-key: YOUR-API-KEY' "https://wrenderer.example.com/admin/renders"
```
**Query Parameters**
- **domain:** Show only caches under the given domain

### List job caches (admin only)

> Note: Currently implement in local build type only

```bash
curl -H 'x-api-key: YOUR-API-KEY' "https://wrenderer.example.com/admin/jobs"
```
**Query Parameters**
- **category:** Show only job caches under the given category type
    - Available values: `sitemap`

### Show configuration settings (admin only)

> Note: Currently implement in local build type only

```bash
curl -H 'x-api-key: YOUR-API-KEY' "https://wrenderer.example.com/admin/config"
```

### Note

url passed to `url` parameter should be encoded for parsing to work correctly

## Build image

```bash
docker build -t image-name:tag .
```

## Setup

1. Create ECR with CloudFormation template
   `setup/aws-lambda/wrenderer-img.yaml`. Given parameter `WrendererName` as
   `${Name}`, this will create an ECR repository with name `${Name}-img`.

   - **Stack name:** `${Name}-img`
     - **Eg.** `wrenderer-example-img`
   - **Parameters:**
     - **WrendererName:** `${Name}`
       - **Eg.** `wrenderer-example`

1. Upload Wrenderer image to ECR repository created in step 1.
1. Create wrenderer lambda with api gateway along with iam role permission using
   CloudFormation template `setup/aws-lambda/wrenderer-main.yaml`.

   - **Stack name:** `${Name}`
     - **Eg.** `wrenderer-example`
   - **Parameters:**
     - **WrendererName:** `${Name}`
       - **Eg.** `wrenderer-example`
     - **WrendererImageDigest:** ECR image digest
       - **Eg.** `sha256:...`
     - Other parameters can be left as default or changed as required,
       `WrendererFunctionMemory` is recommended to be at least set to `4096`

1. Create certificate for CloudFront (Required only if no certificate exist in
   `us-east-1` ACM)

   1. Set region to use `us-east-1`
   1. Create certificate with CloudFormation template
      `setup/aws-lambda/wrenderer-acm.yaml`
      - **Stack name:** `${Name}-acm`
        - **Eg.** `wrenderer-example-acm`
      - **Parameters:**
        - **WrendererName:** `${Name}`
          - **Eg.** `wrenderer-example`
        - **WrendererDomain:** Domain which wrenderer will be hosted
          - **Eg.** `wrenderer.example.com`
        - **WrendererRootDomain:** Root domain of `WrendererDomain`
          - **Eg.** `example.com`
   1. CloudFormation will pause for checking dns validation, set DNS record for
      certificate validation and the progress will continue.

1. Create CloudFront Distribution with CloudFormation template
   `setup/aws-lambda/wrenderer-cdn.yaml` (In `us-east-1` region).

   - **Stack name:** `${Name}-cdn`
   - **Parameters:**
     - **WrendererName:** `${Name}`
       - **Eg.** `wrenderer-example`
     - **WrendererApiDomain:** Api gateway domain associate with the wrenderer
       lambda function. Value be found in the output `WrendererRestApiDomain` of
       the wrenderer main stack.
     - **WrendererApiStage:** Api gateway stage associate with the wrenderer
       lambda function. Value can be found in the output
       `WrendererApiDeploymentStage` of the wrenderer main stack.
     - **WrendererBucketDomain:** S3 bucket's domain that stored output cache
       contents of the wrenderer. Value can be found in the output
       `WrendererBucketRegionalDomain` of the wrenderer main stack.
     - **WrendererCertificateArn:** ARN of the created certificate for
       wrenderer. Value can be found in the output `WrendererCertificateArn` of
       the wrenderer acm stack.
     - **WrendererDomain:** Domain which wrenderer will be hosted. Same as
       `WrendererDomain` in the wrenderer acm stack.

1. Set DNS record to point domain (`WrendererDomain`) to CloudFront distribution
   domain. Value can be found in the output `WrendererDistributionDomain` of the
   wrenderer cdn stack.
