# Web Pipr (piper)

Webpipr allows you to send one request to it and wait for a second request to hit it. It can be used for services like [TelAPI](http://telapi.com), to pass XML to a call request.

http://webpipr.com/new/ will get you a new url to listen on.


## Basic Usage

POST and GET params are passed to the waiting request.

 * Wait for callback: Send request to http://webpipr.com/out/somerandomstring
 * Send callback: Send request to http://webpipr.com/in/somerandomstring

#### Window 1

    $ curl webpipr.com/out/somerandomstring
    last=3
    first=1

#### Window 2

    $ curl 'webpipr.com/in/somerandomstring?first=1&last=3'



## Timeouts

Use `--max-time` or `-m` for `curl` to timeout if it hasn't received the callback in a set amount of time.

    $ curl -m 5 webpipr.com/out/anotherrandomstring
    curl: (28) Operation timed out after 5005 milliseconds with 0 out of -1 bytes received



## Parameter formats

You may append a file extension to the url to set the content type. For the waiting request, if `.json` is appended, the outpu will be encoded as json.

#### Window 1

    $ curl --max-time 30 webpipr.com/out/parametertest.json
    {
        "first": "1",
        "last": "3"
    }

#### Window 2

    $ curl 'webpipr.com/in/parametertest?first=1' -d 'last=3'



## Source content

Use `--data-binary @-` to tell `curl` to send the piped in stdin to the callback request.

#### Window 1
    
    $ echo '<Response><Say>Hello worl</Say></Response>' | curl --data-binary @- webpipr.com/out/customcontent

![Callee](https://raw.githubusercontent.com/mattwilliamson/webpipr/master/callee.gif)

#### Window 2
    
    $ curl 'webpipr.com/in/customcontent.xml'
    <Response><Say>Hello worl</Say></Response>

![Caller](https://raw.githubusercontent.com/mattwilliamson/webpipr/master/caller.gif)