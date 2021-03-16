<!DOCTYPE html>
<% for (var i in htmlWebpackPlugin.files.sprites) { %>
{{- _registerAsset $ "icons" "/assets/<%= i %>" -}}
<% } %>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>{{ block "title" . }}{{ end }} - Readeck</title>
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <meta name="x-csrf-token" content="{{ .csrfToken }}" />
  <meta name="x-api-url" content="{{ urlFor . "/api" }}" />
  <% for (var i in htmlWebpackPlugin.files.sprites) { %>
  <meta name="x-icons" content="{{ .basePath }}assets/<%= i %>" />
  <% } %>
<% for (var i in htmlWebpackPlugin.files.css) { %>
  <link href="{{ .basePath }}<%= htmlWebpackPlugin.files.css[i] %>" rel="stylesheet">
<% } %>
  {{ block "head" . }}{{ end }}
</head>

<body>
{{ block "body" . }}{{ end }}

<% for (var i in htmlWebpackPlugin.files.js) { %>
<script src="{{ .basePath }}<%= htmlWebpackPlugin.files.js[i] %>"></script>
<% } %>
</body>
</html>
