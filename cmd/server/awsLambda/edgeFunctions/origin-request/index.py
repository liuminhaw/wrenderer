import json
import urllib.parse
import requests


WRENDERER_TOKEN = 'Your wrenderer api token'
WRENDERER_DOMAIN = 'your.wrenderer.domain'


def lambda_handler(event, context):
    request = event['Records'][0]['cf']['request']
    headers = request['headers']

    x_target = headers['x-target'][0]['value']
    x_should_render = headers['x-should-render'][0]['value']

    print('Input event: ' + json.dumps(event))

    if x_should_render == 'true':
        x_target_encoded = urllib.parse.quote(x_target, safe='')
        resp = requests.get(
            f'https://{WRENDERER_DOMAIN}/render?url={x_target_encoded}',
            headers={'x-api-key': WRENDERER_TOKEN}
        )
    else:
        return request

    if resp.status_code != 200:
        print(f'Error code: {resp.status_code}, message: {resp.text}')
        return request

    # Update request to rendered result in s3
    resp_data = resp.json()
    rendered_path = resp_data['path']

    headers['host'] = [
        {
            'key': 'host',
            'value': WRENDERER_DOMAIN
        }
    ]
    request['origin'] = {
        'custom': {
            'protocol': 'https',
            'domainName': WRENDERER_DOMAIN,
            'port': 443,
            'path': '',
            'sslProtocols': ['TLSv1.1', 'TLSv1.2'],
            'readTimeout': 30,
            'keepaliveTimeout': 30,
            'customHeaders': {}
        }
    }
    request['querystring'] = ''
    request['uri'] = f'/{rendered_path}'
    print('Output result: ' + json.dumps(request))
    return request
