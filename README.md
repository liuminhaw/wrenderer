# Wrenderer

## Usage
### Render
```bash
curl -H 'x-api-key: YOUR-API-KEY' "https://wrenderer.example.com/render?url=https://www.target.com"
```
### Cache invalidation
Invalidate single url
```bash
curl -X POST -H 'x-api-key: YOUR-API-KEY' "https://wrenderer.example.com/render?url=https://www.target.com" 
```
Invalidate whole domain
```bash
curl -X DELETE -H 'x-api-key: YOUR-API-KEY' "https://wrenderer.example.com/render?domain=www.target.com"
```

### Note
url passed to `url` parameter should be encoded for parsing to work correctly

## Build image
```bash
docker build -t image-name:tag .
```

## Setup
1. Create ECR with CloudFormation template `setup/aws-lambda/wrenderer-img.yaml`
    - Stack name: `${Name}-img`
    - Parameters:
        - WrendererName: `${Name}`
1. Upload Wrenderer image to created repository
1. Create main resources with CloudFormation template `setup/aws-lambda/wrenderer-main.yaml`
    - Stack name: `${Name}`
    - Parameters:
        - WrendererName: `${Name}`
        - WrendererImageDigest: ECR image digest (sha256:...)
1. Create certificate for CloudFront (Only required if no certificate exist in us-east-1 ACM)
    1. Set region to use `us-east-1`
    1. Create certificate with CloudFormation template `setup/aws-lambda/wrenderer-acm.yaml`
        - Stack name: `${Name}-acm`
        - Parameters:
            - WrendererName: `${Name}`
    1. Set DNS record on for certificate validation
1. Create CloudFront Distribution with CloudFormation template `setup/aws-lambda/wrenderer-cdn.yaml`
    - Stack name: `${Name}-cdn`
    - Parameters:
        - WrendererName: `${Name}`
1. Set DNS record to point domain to CloudFront distribution domain
        

