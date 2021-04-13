// Infer a correct type for deviantart pages.
function deviantartPage() {
  // There's a JSON URL in the collected link tags
  let link = (drop.Meta["link.alternate"] || []).find(function(x) {
    return x.match(/format=json$/)
  })
  if (!link) {
    return
  }

  // This link is weirdly encoded, let's fix it and fetch the
  // JSON payload.
  link = $.unescapeURL(link)
  let node = $.fetchJSON(link)

  // Get author's name
  let author = node.Get("$.author_name")
  if (author) {
    drop.SetAuthors(author)
  }

  // Get the publication date
  let date = node.Get("$.pubdate")
  if (date) {
    drop.SetMeta("html.date", date)
  }

  // If it's a photo, change the type and the picture URL
  let type = node.Get("$.type")
  if (type == "photo") {
    drop.SetDocumentType("photo")

    let img = node.Get("$.url")
    if (img) {
      drop.SetMeta("x.picture_url", $.unescape(img))
      console.debug({"url": img}, "set picture")
    }
  }
}

// Pinterest. A pin is an image (and a link to somewhere). Set it as a photo
// and call it a day.
function pinterestImage() {
  if (!drop.URL.Path.match(/^\/pin\//)) {
    return
  }

  let img = drop.Meta["graph.image"]
  if (img && img.length > 0) {
    drop.SetDocumentType("photo")
    drop.SetMeta("x.picture_url", img)
  }
}

// Reddit posts
function redditPost() {
  // Any reddit post provides a JSON payload when you add
  // a ".json" extension to it. Pretty neat.
  let url = drop.URL.String() + ".json"
  let node = $.fetchJSON(url)
  let postID = node.Get("$[0].data.children[0].data.name")
  let postHint = node.Get("$[0].data.children[0].data.post_hint")

  // Fetch post information
  let post = $.fetchJSON("https://gateway.reddit.com/desktopapi/v1/postcomments/" + postID)

  if (postHint == "image") {
    // We have a picture!
    drop.SetDocumentType("photo")
    let img = post.Get("$.posts["+postID+"].media.resolutions[(@.length-1)].url")
    drop.SetMeta("x.picture_url", $.unescape(img))
    console.debug({"url": img}, "set picture")

    let title = post.Get("$.posts["+postID+"].title")
    if (title) {
      drop.SetTitle(title)
    }

    let author = post.Get("$.posts["+postID+"].author")
    if (author) {
      drop.SetAuthors(author)
    }
    drop.SetDescription("")
  }
}

// Unsplash, only set a document type "photo"
// on the /photo/* path
function unsplashPhoto() {
  if (!drop.URL.Path.match(/^\/photos\//)) {
    return
  }
  drop.SetDocumentType("photo")

  // There's a JSON endpoint with plenty of goodies
  let url = drop.URL
  url.Path = "/napi" + drop.URL.Path
  let node = $.fetchJSON(url.String())

  let author = node.Get("$.user.name")
  if (author) {
    drop.SetAuthors(author)
  }

  let title = node.Get("$.description")
  if (title) {
    drop.SetTitle(title)
  }

  let desc = node.Get("$.alt_description")
  if (title) {
    drop.SetDescription(desc)
  }

  let date = node.Get("$.created_at")
  if (date) {
    drop.SetMeta("html.date", date)
  }
}

// Vimeo big thumbnail includes a play button.
// The actual picture URL is given by one of the URL
// query parameters (src0)
function vimeoThumbnail() {
  let twPicture = drop.Meta["graph.image"]
  if (twPicture.length == 0) {
    return
  }

  let url = $.parseURL(twPicture[0])
  let img = url.Query().Get("src0")
  if (img != "") {
    drop.SetMeta("x.picture_url", img)
    console.debug({"url": img}, "set picture")
  }
}

!(function() {
  switch (drop.Domain) {
    case "deviantart.com":
      deviantartPage()
      break

    case "pinterest.com":
      pinterestImage()
      break

    case "reddit.com":
      redditPost()
      break

    case "unsplash.com":
      unsplashPhoto()
      break

    case "vimeo.com":
      vimeoThumbnail()
      break
  }
})()
