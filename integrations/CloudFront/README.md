# Wrenderer CloudFront Integration

## Setup

1. Use CloudFormation template `edgeFunctions/wrenderer-edgefunction.yaml` to
   create Lambda function prerequisite for using in CloudFront Lambda@Edge. This
   will create role for Lambda function along with viewer request and origin
   request function skeleton itself. (Should be created in `us-east-1` region)

   - **Stack name:** `${Name}-edgefunctions`
     - **Eg.** `wrenderer-example-edgefunctions`
   - **Parameters:**
     - **WrendererName:** `${Name}`
       - **Eg.** `wrenderer-example`

1. Pack and upload viewer request lambda code

   - Pack viewer request lambda function code into zip file
     ```bash
     # Execute command below inside directory edgeFunctions/viewer-request
     zip wrenderer-viewer-request.zip index.py
     ```
   - Upload generated zip file to viewer request lambda function
   - Publish new version of viewer request lambda function for using in
     CloudFront

1. Pack and upload origin request lambda code

   - Substitute `WRENDERER_TOKEN` and `WRENDERER_DOMAIN` in `index.py` with
     actual value
     - `WRENDERER_TOKEN` is the api key created in api gateway used for
       authentication in Wrenderer. Value can be found from the Api Gateway used
       by wrenderer lambda function.
     - `WRENDERER_DOMAIN` is the domain used for hosting wrenderer. This is same
       as the `WrendererDomain` used when creating the certificate and
       CloudFront with CloudFormation template.
       - **Eg.** `wrenderer.example.com`
   - Pack origin request lambda function code into zip file
     ```bash
     # Execute command below inside directory edgeFunctions/origin-request
     mkdir package
     pip3 install --target ./package requests
     cd package && zip -r ../wrenderer-origin-request.zip . && cd ..
     zip wrenderer-origin-request.zip index.py
     ```
   - Upload generated zip file to origin request lambda function
   - Publish new version of origin request lambda function for using in
     CloudFront

1. Configure the CloudFront behaviors with the ARNs for the view request and
   origin request Lambda function versions, along with the cache policy and
   origin request policy created by the CloudFormation template, which are
   required for bot detection and rendering.
