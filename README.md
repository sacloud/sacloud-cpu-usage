# sacloud-cpu-usage

Fetch cpu usage of server instances from sacloud API and calcurate Max/Min/Average.

# Usage

```
Usage:
  sacloud-cpu-usage [OPTIONS]

Application Options:
      --time=           Get average CPU usage for a specified amount of time (default: 3)
      --prefix=         prefix for server names. prefix accepts more than one.
      --zone=           zone name
      --percentile-set= percentiles to dispaly (default: 99,95,90,75)
  -v, --version         Show version
      --query=          jq style query to result and display
      --env-from=       load envrionment values from this file

Help Options:
  -h, --help            Show this help message
```


### Example

```
% ./sacloud-cpu-usage --zone tk1b --prefix example --prefix dev --time 3 --query '.avg'            
2021/09/22 10:53:32 example-s1 cores:2 cpu:0.170000 time:2021-09-22 10:40:00 +0900 JST
2021/09/22 10:53:32 example-s1 cores:2 cpu:0.166667 time:2021-09-22 10:45:00 +0900 JST
2021/09/22 10:53:32 example-s1 cores:2 cpu:0.166667 time:2021-09-22 10:50:00 +0900 JST
2021/09/22 10:53:32 example-s1 avg:8.388889
2021/09/22 10:53:32 example-s2 cores:2 cpu:0.230000 time:2021-09-22 10:40:00 +0900 JST
2021/09/22 10:53:32 example-s2 cores:2 cpu:0.236667 time:2021-09-22 10:45:00 +0900 JST
2021/09/22 10:53:32 example-s2 cores:2 cpu:0.226667 time:2021-09-22 10:50:00 +0900 JST
2021/09/22 10:53:32 example-s2 avg:11.555556
2021/09/22 10:53:33 example-s3 cores:2 cpu:0.126667 time:2021-09-22 10:40:00 +0900 JST
2021/09/22 10:53:33 example-s3 cores:2 cpu:0.136667 time:2021-09-22 10:45:00 +0900 JST
2021/09/22 10:53:33 example-s3 cores:2 cpu:0.126667 time:2021-09-22 10:50:00 +0900 JST
2021/09/22 10:53:33 example-s3 avg:6.500000
2021/09/22 10:53:34 dev1 cores:2 cpu:0.030000 time:2021-09-22 10:40:00 +0900 JST
2021/09/22 10:53:34 dev1 cores:2 cpu:0.030000 time:2021-09-22 10:45:00 +0900 JST
2021/09/22 10:53:34 dev1 cores:2 cpu:0.030000 time:2021-09-22 10:50:00 +0900 JST
2021/09/22 10:53:34 dev1 avg:1.500000
2021/09/22 10:53:34 {"75pt":8.388888888999999,"90pt":11.555555555666666,"95pt":11.555555555666666,"99pt":11.555555555666666,"avg":6.986111111208333,"count":4,"max":11.555555555666666,"min":1.5}
6.986111111208333
```


# 使用例

https://kazeburo.hatenablog.com/entry/2021/09/27/102524
