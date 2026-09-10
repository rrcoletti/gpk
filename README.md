# gpk

A terminal kanban board for your GitHub Projects v2. Your project's items show up in status columns, the same way the browser displays them, and you can move cards between columns without leaving the terminal. Run it with no arguments for the full flow, or `./gpk --mock` to look at the board with sample data.

There is nothing else to install. No `gh`, and the only browser visit is a one-time auth.

## What it does

- Authenticates once via the GitHub device flow. You approve in the browser, the token is stored locally.
- Lists your GitHub Projects v2 projects and lets you pick one.
- Renders the project as a kanban board, one column per Status option, in the same order as the web UI.
- Shows items (issues, pull requests, draft issues) with number and assignee.
- Moves items between columns with two keys, `H` and `L`, and writes the change to GitHub.
- Re-fetches items every 5 seconds, so changes made in the browser or by teammates appear on their own.
- Talks to GitHub over the GraphQL API with a plain `net/http` client.

## Building

Go 1.24 or newer. The build is pure Go, no C compiler.

**Linux:** Install Go from your package manager, for example:

    # Debian / Ubuntu
    sudo apt install golang-go

    # Fedora
    sudo dnf install golang

    # Arch
    sudo pacman -S go

Distro Go packages can lag behind. If yours is older than 1.24, grab the latest from [go.dev/dl](https://go.dev/dl/) instead.

**macOS:** Install with Homebrew:

    brew install go

Then build:

    git clone https://github.com/rrcoletti/gpk.git
    cd gpk
    go build

That creates the `gpk` binary.

### Installing the binary

You can leave `gpk` in the repo or move it somewhere on your `$PATH`:

    mv gpk ~/bin/

    # or
    mv gpk ~/.local/bin/

## First run

Start the app with no arguments:

    ./gpk

On the first run there is no GitHub token, so gpk starts the device flow:

1. It prints a URL (`https://github.com/login/device`) and a short code like `ABCD-1234`
2. Open the URL in a browser (most terminals make the printed URL clickable), enter the code
3. Authorize gpk. It asks for repository access and projects read/write.
4. The app picks up the approval on its own and saves the token.

The token is stored in `~/.config/gpk/env` with mode 0600, in a 0700 directory:

    GITHUB_TOKEN=ghp_yourtokenhere

Every later run reads that file and goes straight to the project picker. To re-authenticate, for example after a scope change, delete the file and run gpk again. A `GITHUB_TOKEN` environment variable takes precedence over the file.

## Using the board

The project picker lists your projects, most recently updated first. Move with `j`/`k` or the arrow keys and press `Enter` to open one as a board.

On the board:

    h/l or ←/→     move between columns
    j/k or ↑/↓     move between cards
    g / G          jump to first/last card in the column
    H              move the selected card one column left
    L              move the selected card one column right
    q              quit

`H` and `L` change the item's Status field on GitHub, the same as dragging the card in the browser. Moving a card into the leftmost "No Status" column clears its status. Items whose status option was deleted from the field also land in "No Status".

The board re-fetches items every 5 seconds. Move a card in the browser while the TUI is open and it appears on its own within five seconds. Your selected card stays selected across refreshes.

Two other flags: `./gpk --mock` shows the board with sample data and makes no API calls, and `./gpk --whoami` prints the login the stored token resolves to.

## Things worth knowing

- The device flow grants the classic coarse OAuth scopes, `repo` and `project`. `repo` means full read and write on your private repositories. That is broader than gpk needs, but it is the only granularity this auth method offers.
- Moves are optimistic. The card moves in the UI right away, and if GitHub rejects the change you get an error message while the next refresh restores the real state.
- The 5-second refresh sends one GraphQL request per tick, about 720 per hour on an open board. GitHub allows 5000 per hour for authenticated requests, so a single instance is fine; a handful of open boards is still fine, but it adds up.
- Draft issues have no number and no repository, so they render with just their title.

## License

This program is free software: you can redistribute it and/or modify it under the terms of the GNU General Public License as published by the Free Software Foundation, either version 3 of the License or (at your option) any later version.

This program is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the [LICENSE](LICENSE) for more details.
