import { Controller } from "stimulus"
import api from "../lib/api"
import $ from "../lib/dq"

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

    this.tagTargets.forEach(e => {
      if (!rsp.tags.includes($("button", e).getAttr("value"))) {
        $(e).remove()
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
    $("li", tpl).prepend($.T(name))
    $("button", tpl).attr("value", name)
    $(this.tagListTarget).prepend(tpl)
  }

  // clearTagList removes all the tag elements in the list.
  clearTagList() {
    this.tagTargets.forEach(e => {
      $(e).remove()
    })
  }
};
