import assert from 'node:assert/strict';
import { tavily } from '@tavily/core';
import Exa, { DYNAMIC_HIGHLIGHTS_BETA } from 'exa-js';

const root = process.env.WEB_TEST_URL;
const key = process.env.WEB_TEST_KEY;
const dialect = process.env.WEB_TEST_DIALECT;
const page = 'https://go.dev/doc/';

if (dialect === 'tavily') {
  const client = tavily({ apiKey: key, apiBaseURL: `${root}/v1/tavily` });
  const search = await client.search('Go programming language documentation', {
    maxResults: 1, includeDomains: ['go.dev'], includeAnswer: false, includeRawContent: true,
  });
  assert.equal(search.results.length, 1);
  assert.equal(new URL(search.results[0].url).hostname, 'go.dev');
  assert.ok(search.results[0].content.length > 0);
  assert.match(search.results[0].rawContent, /Go/);
  assert.ok(search.requestId);
  const extraction = await client.extract([page], { timeout: 30 });
  assert.equal(extraction.results.length, 1);
  assert.equal(extraction.failedResults.length, 0);
  assert.match(extraction.results[0].rawContent, /Go/);
} else {
  const client = new Exa(key, `${root}/v1/exa`);
  const search = await client.search('Go programming language documentation', {
    type: 'lite', numResults: 1, includeDomains: ['go.dev'],
    contents: { text: true, highlights: { dynamic: true } }, betas: [DYNAMIC_HIGHLIGHTS_BETA],
  });
  assert.equal(search.results.length, 1);
  assert.equal(new URL(search.results[0].url).hostname, 'go.dev');
  assert.ok(search.requestId);
  assert.match(search.results[0].text, /Go/);
  assert.ok(search.results[0].highlights.some(text => text.length > 0));
  const extraction = await client.getContents([page], { text: { maxCharacters: 1000 } });
  assert.equal(extraction.results.length, 1);
  assert.match(extraction.results[0].text, /Go/);
  assert.ok([...extraction.results[0].text].length <= 1000);
}
console.log(`${dialect}: official JavaScript SDK search and extraction passed`);
