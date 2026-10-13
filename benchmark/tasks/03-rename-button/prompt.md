In the web UI (`web/`), rename the note form's submit button label from "Add note" to "Create note".

After the change, `render(<App />)` must show a button with accessible name `Create note` and no button named `Add note`. Update existing tests that referenced the old label so the whole suite stays green.
