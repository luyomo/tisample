global:
  user: "admin"
  ssh_port: 22
  deploy_dir: "/home/admin/dm/dm-deploy"
  data_dir: "/home/admin/dm/dm-data"

monitored:
  node_exporter_port: 19100
  blackbox_exporter_port: 19115
  deploy_dir: /home/admin/dm/dm-deploy/monitor-19100
  data_dir: /home/admin/dm/dm-data/monitor-19100
  log_dir: /home/admin/dm/dm-deploy/monitor-19100/log

server_configs:
  master:
    log-level: info
  worker:
    log-level: info

master_servers:
   {{- range .DM }}
  - host: {{.PrivateIpAddress }}
    name: master1
    ssh_port: 22
    port: 8261
    config:
      log-level: info
   {{- end }}

worker_servers:
   {{- range .DM }}
  - host: {{.PrivateIpAddress }}
    ssh_port: 22
    port: 8262
    config:
      log-level: info
   {{- end }}

monitoring_servers:
   {{- range .Monitor }}
  - host: {{.PrivateIpAddress }}
    port: 19090
   {{- end }}
grafana_servers:
   {{- range .Monitor }}
  - host: {{.PrivateIpAddress }}
    port: 13000
   {{- end }}
alertmanager_servers:
   {{- range .Monitor }}
  - host: {{.PrivateIpAddress }}
    web_port: 19093
    cluster_port: 19094
   {{- end }}
