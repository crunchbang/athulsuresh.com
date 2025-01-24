+++
title = "Faster Multitasking with Hammerspoon"
author = ["Athul Suresh"]
date = 2021-07-11
draft = false
+++

When I'm working, I usually have at-least 4 windows open including Slack, Chrome, iTerm and a few other stuff. Lately, I've noticed that switching between them via `cmd+tab` is turning out to be a waste of time, since the order of the apps keep changing based on how recently they were used.

I wanted a smoother transition between my umpteen apps without getting stuck in `cmd+tab` hell.

I needed static key-bindings!

That's when I found [Hammerspoon](http://www.hammerspoon.org/). Hammerspoon is a neat little tool that allows you to write little automations for a wide range of stuff on Mac. It's completely configurable and supports Lua as the extension langague, which is even cooler. And to top it all off, the documentation is thorough and on-point.

I was able to hack together a tiny script to serve my needs.

`init.lua` for Hammerspoon:

```lua
function open_app(name)
    return function()
        hs.application.launchOrFocus(name)
    end
end

hs.hotkey.bind({"cmd"}, "1", open_app("iTerm"))
hs.hotkey.bind({"cmd"}, "2", open_app("Visual Studio Code - Insiders"))
hs.hotkey.bind({"cmd"}, "3", open_app("Visual Studio Code"))
hs.hotkey.bind({"cmd"}, "4", open_app("Slack"))
hs.hotkey.bind({"cmd"}, "5", open_app("Preview"))
hs.hotkey.bind({"cmd"}, "7", open_app("MacVim"))
hs.hotkey.bind({"cmd"}, "8", open_app("Google Chrome"))
hs.hotkey.bind({"cmd"}, "9", open_app("Firefox"))
hs.hotkey.bind({"cmd"}, "'", open_app("Spotify"))
hs.hotkey.bind({"cmd"}, ";", open_app("Postman"))
```

This might seem trivial and even borderline insane, but the payout has been immense with a smoother workflow with fewer impediments.

This was inspired by [jasonrudolph/keyboard](https://github.com/jasonrudolph/keyboard) who has done some pretty cool stuff with Hammerspoon.
