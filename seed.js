import {JSONFilePreset} from 'lowdb/node'

function upsert(array, item, key) {
  const index = array.findIndex(i => i[key] === item[key])
  if (index === -1) {
    // insert new item
    array.push(item)
  } else {
    // update existing item
    array[index] = item
  }
}

async function* scanner(url) {
  while(url) {
    const res = await fetch(url)
    if (!res.ok) return

    const body = await res.json()
    yield body

    url = body.next
  }
}

const p = await JSONFilePreset("publishers.json", { publishers: [] })
const b = await JSONFilePreset("books.json", { books: [] })

const getPublishers = async () => {
  const limit = 100
  const url = `https://learning.oreilly.com/api/v1/publishers?limit=${limit}`

  for await(const { results } of scanner(url)) {
    console.log(`processing ${results.length} publishers`)
    p.update(({publishers}) => results.forEach(publisher =>  upsert(publishers, publisher, "uuid")))
  }
}

await getPublishers()

const getBooksByPublisher = async({uuid}) => {
  const limit = 100
  const url = `https://learning.oreilly.com/api/v2/metadata/?publisher_uuid=${uuid}&limit=${limit}&offset=0&type=book&sort=-publication_date&language=en`

  for await(const {results} of scanner(url)) {
    console.log(`processing ${results.length} books`)
    b.update(({books}) => results.forEach(book => upsert(books, book, "ourn")))
  }
}

for (const publisher of p.data.publishers) {
  await getBooksByPublisher(publisher)
}
