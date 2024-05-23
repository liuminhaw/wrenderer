import json


def lambda_handler(event, context):
    request = event['Records'][0]['cf']['request']
    headers = request['headers']
    user_agent = headers['user-agent'][0]['value']
    host = headers['host'][0]['value']
    querystring = request['querystring']
    uri = request['uri']

    print('Input event: ' + json.dumps(event))

    if querystring == '':
        uri_with_querystring = uri
    else:
        uri_with_querystring = f'{uri}?{querystring}'

    headers['x-target'] = [
        {
            'key': 'x-target',
            'value': f'https://{host}{uri_with_querystring}'
        }
    ]

    if check_if_bot(user_agent) and not check_suffix_match(uri):
        headers['x-should-render'] = [
            {
                'key': 'x-should-render',
                'value': 'true'
            }
        ]
    else:
        headers['x-should-render'] = [
            {
                'key': 'x-should-render',
                'value': 'false'
            }
        ]

    print('Output result: ' + json.dumps(request))

    return request


def check_if_bot(user_agent):
    bot_agents = (
        'googlebot',
        'APIs-Google',
        'Mediapartners-Google',
        'AdsBot-Google',
        'Mediapartners-Google',
        'FeedFetcher-Google',
        'Google-Read-Aloud',
        'DuplexWeb-Google',
        'googleweblight',
        'Storebot-Google',
        'Yahoo! Slurp',
        'bingbot',
        'yandex',
        'baiduspider',
        'facebookexternalhit',
        'twitterbot',
        'rogerbot',
        'linkedinbot',
        'embedly',
        'quora link preview',
        'showyoubot',
        'outbrain',
        'pinterest/0.',
        'developers.google.com/+/web/snippet',
        'slackbot',
        'vkShare',
        'W3C_Validator',
        'redditbot',
        'Applebot',
        'WhatsApp',
        'Linespider',
        'flipboard',
        'tumblr',
        'bitlybot',
        'SkypeUriPreview',
        'nuzzel',
        'Discordbot',
        'Google Page Speed',
        'Qwantify',
        'pinterestbot',
        'Bitrix link preview',
        'XING-contenttabreceiver',
        'Chrome-Lighthouse',
        'AhrefsBot',
        'AhrefsSiteAudit'
    )

    user_agent = user_agent.lower()
    for agent in bot_agents:
        if agent.lower() in user_agent:
            return True
        else:
            continue
    return False


def check_suffix_match(uri):
    suffixes = (
        '.js',
        '.css',
        '.xml',
        '.less',
        '.png',
        '.jpg',
        '.jpeg',
        '.gif',
        '.pdf',
        '.doc',
        '.txt',
        '.ico',
        '.rss',
        '.zip',
        '.mp3',
        '.rar',
        '.exe',
        '.wmv',
        '.avi',
        '.ppt',
        '.mpg',
        '.mpeg',
        '.tif',
        '.wav',
        '.mov',
        '.psd',
        '.ai',
        '.xls',
        '.mp4',
        '.m4a',
        '.swf',
        '.dat',
        '.dmg',
        '.iso',
        '.flv',
        '.m4v',
        '.torrent',
        '.ttf',
        '.woff',
        '.svg',
        '.eot'
    )

    uri = uri.lower()
    for suffix in suffixes:
        if uri.endswith(suffix.lower()):
            return True
        else:
            continue
    return False
