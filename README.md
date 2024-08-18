# Frieren Bot

![Frieren](profile.png)

Checks machine's temperature via `sensors(1)` and sends a message to a Discord
channel with the `sensors(1)` and `top(1)` output.

This is not a daemon: the program starts, do what it have to and leaves.

## Usage

Spawn the program with the environment variable `DISCORD_TOKEN` set to the
appropriate Discord bot token. Pass the channel ID (the bot must be a member of
the guild) via the `-channelID` option and the threshold temperature
(in Celsius\[1\]\[2\]\[3\]) via the `-limit` option.

\[1\] - [Obligatory XKCD](https://xkcd.com/1643/)

\[2\] - [Another obligatory XKCD](https://www.xkcd.com/1923/)

\[3\] - [Yet another obligatory XKCD](https://www.xkcd.com/2292/)
