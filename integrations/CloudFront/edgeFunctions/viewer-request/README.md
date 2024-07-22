# Viewer Request Lambda@Edge

## Package creation

Steps to package code for uploading to lambda function as lambda edge usage

1. Compress the main lambda function file into zip file
   ```bash
   zip wrenderer-viewer-request.zip index.py
   ```

### Reference

- [aws docs - python package native libraries](https://docs.aws.amazon.com/lambda/latest/dg/python-package.html#python-package-native-libraries)
