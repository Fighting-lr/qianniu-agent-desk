import assert from "node:assert/strict"
import test from "node:test"
import { readFile } from "node:fs/promises"

async function loadMessages(locale) {
  const source = await readFile(new URL(`../messages/${locale}.json`, import.meta.url), "utf8")
  return JSON.parse(source)
}

test("legal pages have localized document content", async () => {
  for (const locale of ["zh-CN", "en-US"]) {
    const messages = await loadMessages(locale)

    assert.equal(typeof messages.legal.terms.title, "string")
    assert.equal(typeof messages.legal.privacy.title, "string")
    assert.ok(messages.legal.terms.sections.length >= 6)
    assert.ok(messages.legal.privacy.sections.length >= 6)

    for (const page of [messages.legal.terms, messages.legal.privacy]) {
      assert.equal(typeof page.updatedAt, "string")
      assert.equal(typeof page.relatedLabel, "string")
      assert.equal(typeof page.relatedLink, "string")
      for (const section of page.sections) {
        assert.equal(typeof section.title, "string")
        assert.equal(typeof section.body, "string")
      }
    }
  }
})
