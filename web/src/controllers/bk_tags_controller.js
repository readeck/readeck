import { Controller } from "stimulus"
import api from "../lib/api"

export default class extends Controller {
  static get targets() {
    return  ["tag", "tagList", "newTag", "tmpl"]
  }
  static get values() {
    return {
      id: String,
    }
  }

  // removeTag removes a tag from the bookmark.
  async removeTag(evt) {
    evt.preventDefault()
    let data = {remove_tags: [evt.target.value]}

    let rsp = await api.patchJSON(`bookmarks/${this.idValue}`, data)

    this.tagTargets.forEach(el => {
      let b = el.querySelector("button")
      if (!rsp.tags.includes(b.value)) {
        el.parentNode.removeChild(el)
      }
    })
  }

  // addTag adds a new tag to the bookmark and makes a new tag list
  // based on the new tag list received from the api request.
  async addTag(evt) {
    if (!this.hasNewTagTarget) {
      return
    }
    evt.preventDefault()

    if (this.newTagTarget.value == "") {
      return
    }

    let data = {add_tags: [this.newTagTarget.value]}
    let rsp = await api.patchJSON(`bookmarks/${this.idValue}`, data)

    this.clearTagList()
    rsp.tags.reverse().forEach(t => this.addTagElement(t))

    this.newTagTarget.value = ""
    this.newTagTarget.focus()
  }

  // addTagElement adds a new tag element to the list, using
  // the provided template.
  addTagElement(name) {
    let tpl = this.tmplTarget.content.cloneNode(true)
    let li = tpl.querySelector("li")
    li.insertBefore(document.createTextNode(name), li.firstChild)
    tpl.querySelector("button").value = name

    this.tagListTarget.insertBefore(tpl, this.tagListTarget.firstChild)
  }

  // clearTagList removes all the tag elements in the list.
  clearTagList() {
    this.tagTargets.forEach(e => {
      e.parentNode.removeChild(e)
    })
  }
};
