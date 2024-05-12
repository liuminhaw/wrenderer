# Wrenderer

## Note
url passed to `url` parameter should be encoded for parsing to work correctly

## Build image
```bash
docker build -t image-name:tag .
```

## Lambda deployment
1. Upload image to ECR
1. Create **Container Image** type Lambda function
