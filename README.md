## Plex-Tuner

兼容Plex的电视直播和数字录像机软件



#### 配置文件

```json
{
    "id": "iptv",
    "tuner_count": 1000,
    "listen": "0.0.0.0:33400",
    "channel": "channel.json"
}
```

id：同一个plex中，id不能重复

tuner_count：plex中同时可以播放的数量（实际上并不限制）

listen：监听的端口

channel：频道列表文件，支持http/https地址作为源



#### 频道列表文件

```json
[
    {
        "id": "bilibili",
        "name": "Bilibili",
        "url": "23900931",
        "type": "bilibili"
    },
    {
        "id": "7",
        "name": "CCTV-6电影",
        "url": "http://stream-shbu.bestvcdn.com:8080/live/program/live/cctv6hd/4000000/mnf.m3u8",
        "type": "hls"
    },
    {
        "id": "camera",
        "name": "IP Camera",
        "url": "rtsp://127.0.0.1:554/h264/ch1/main/av_stream",
        "type": "rtsp"
    }
]
```

id：在plex所配置的epgxml文件中与channel的id要对应上

name：在plex所配置的epgxml文件中与channel的名称要对应上

url：源地址，如果是bilibili，则为Bilibili直播间的id

type：源类型，支持hls、rtsp、bilibili

#### 

#### 开发相关

目前初步测试，plex所支持的流为ts格式的流，mp4f的流似乎无法播放出来。

ts的流可以在流的任意一个位置开始读，mp4f的流由于需要header的信息，所以做不到任意位置读取，需要从header开始位置读取。
