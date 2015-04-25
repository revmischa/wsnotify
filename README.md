WSNotify
========
### WebSocket interface to PostgreSQL [LISTEN](http://www.postgresql.org/docs/9.4/static/sql-listen.html)/[NOTIFY](http://www.postgresql.org/docs/9.4/static/sql-notify.html) message passing functionality

## PostgreSQL message passing? What is this sourcery?
Unbeknownst to many people, the excellent open-source database PostgreSQL can pass around messages on channels to asynchronous listeners. It is a very simple and useful service if you want to push messages and already use Postgres. You can read more about it [here](http://www.postgresql.org/docs/9.4/static/sql-notify.html) and see some demos [here](https://github.com/revmischa/pgnotify-demos) along with some [slides](https://github.com/revmischa/pgnotify-demos/blob/master/slides.pdf) from a talk.

```
LISTEN virtual;
NOTIFY virtual;
Asynchronous notification "virtual" received from server process with PID 8448.
NOTIFY virtual, 'This is the payload';
Asynchronous notification "virtual" with payload "This is the payload" received from server process with PID 8448.
```

## What is WSNotify?
WSNotify is a websocket server. Clients can connect and request to listen on a channel, and can also post events. It essentially is a push notification system using PostgreSQL as the transport. This makes it easy to push messages from other applications directly to any connected clients, in publish-subscribe style.

## Configuration
Copy config.yaml to /etc/wsnotify/config.yaml and edit it to select a port to listen on and your database connection info.

## Postgres clients
To send and receive asynchronous messages through Postgres you can use any Postgres client library, as basically all of them support this capability. You can also use libpq and `select()` for C applications [demo here](https://github.com/revmischa/pgnotify-demos/blob/master/pglisten.c).

## Building
....

## Example
...
