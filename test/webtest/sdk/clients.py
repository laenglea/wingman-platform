import os
from urllib.parse import urlparse

root = os.environ['WEB_TEST_URL']
key = os.environ['WEB_TEST_KEY']
dialect = os.environ['WEB_TEST_DIALECT']
page = 'https://go.dev/doc/'

if dialect == 'tavily':
    from tavily import TavilyClient
    client = TavilyClient(api_key=key, api_base_url=f'{root}/v1/tavily')
    search = client.search('Go programming language documentation', max_results=1,
                           include_domains=['go.dev'], include_raw_content=True)
    assert len(search['results']) == 1
    assert urlparse(search['results'][0]['url']).hostname == 'go.dev'
    assert search['results'][0]['content']
    assert 'Go' in search['results'][0]['raw_content']
    extracted = client.extract([page], timeout=30)
    assert len(extracted['results']) == 1
    assert not extracted['failed_results']
    assert 'Go' in extracted['results'][0]['raw_content']
else:
    from exa_py import Exa
    from exa_py.api import DYNAMIC_HIGHLIGHTS_BETA
    client = Exa(api_key=key, base_url=f'{root}/v1/exa')
    search = client.search('Go programming language documentation', type='lite', num_results=1,
                           include_domains=['go.dev'], contents={'text': True, 'highlights': {'dynamic': True}},
                           betas=[DYNAMIC_HIGHLIGHTS_BETA])
    assert len(search.results) == 1
    assert urlparse(search.results[0].url).hostname == 'go.dev'
    assert 'Go' in search.results[0].text
    assert any(search.results[0].highlights)
    extracted = client.get_contents([page], text={'max_characters': 1000})
    assert len(extracted.results) == 1
    assert 'Go' in extracted.results[0].text
    assert len(extracted.results[0].text) <= 1000

print(f'{dialect}: official Python SDK search and extraction passed')
