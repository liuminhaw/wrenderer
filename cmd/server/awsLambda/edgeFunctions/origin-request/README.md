# Origin Request Lambda@Edge

## Package creation

Steps to package code for uploading to lambda function as lambda edge usage

1. Create new directory named `package` for installing dependencies
   ```bash
   mkdir package
   ```
1. Install `requests` dependency in the `package` directory
   ```bash
   pip3 install --target ./package requests
   ```
1. Create a `.zip` file with the installed libraries at the root
   ```bash
   cd package
   zip -r ../wrenderer-origin-request.zip .
   ```
1. Add the main lambda function file to the root of the zip file
   ```bash
   cd ..
   zip wrenderer-origin-request.zip index.py
   ```

### Reference

- [aws docs - python package native libraries](https://docs.aws.amazon.com/lambda/latest/dg/python-package.html#python-package-native-libraries)
