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

## Lambda deployment
1. Upload image to ECR
1. Create **Container Image** type Lambda function
