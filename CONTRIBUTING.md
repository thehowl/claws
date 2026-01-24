Thank you for your interest in helping out on Claws!

If you're filing a bug report, make sure to provide the full stack trace (if possible), or any result from the console that is unexpected.

If you want to work on a feature you'd deem useful, please make an issue first!

If you have any doubts, please get in touch with me via [Email](mailto:the@howl.moe).

## Code style guidelines

* `gofmt` is required on all code.
* Try to make the linters happy.
* Use lowerCamelCase for local variables and unexported functions, and
  CapitalCamperCase for methods and struct fields which are "exportable". (Since
  this is a single-package command, everything is always accessable regardless
  of un/exportable, but ask yourself: if this were separated in a package what
  would be exported?)
