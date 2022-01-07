global:
  user: "admin"
  ssh_port: 22
  deploy_dir: "/home/admin/tidb/tidb-deploy"
  data_dir: "/home/admin/tidb/tidb-data"
server_configs:
   pd:
     replication.location-labels: ["az"]
{{ if gt (len .TiCDC) 0 }}
  cdc:
    per-table-memory-quota: 20971520
{{ end  }}
{{ if gt (len .PD) 0 }}
pd_servers:
  {{- range .PD }}
  - host: {{.PrivateIpAddress }}
  {{- end }}
{{ end }}
{{ if gt (len .TiDB) 0 }}
tidb_servers:
  {{- range .TiDB }}
  - host: {{.PrivateIpAddress }}
  {{- end }}
{{ end }}
{{ if gt (len .TiKV) 0 }}
tikv_servers:
  {{- range .TiKV }}
  - host: {{.PrivateIpAddress }}
    config:
      server.labels:
        az: {{.Placement.AvailabilityZone }}
  {{- end }}
{{ end  }}
{{ if gt (len .TiCDC) 0 }}
cdc_servers:
  {{- range .TiCDC }}
  - host: {{.PrivateIpAddress }}
  {{- end }}
{{ end  }}
{{ if gt (len .Monitor) 0 }}
monitoring_servers:
  {{- range .Monitor }}
  - host: {{.PrivateIpAddress }}
  {{- end }}
grafana_servers:
  {{- range .Monitor }}
  - host: {{.PrivateIpAddress }}
  {{- end }}
alertmanager_servers:
  {{- range .Monitor }}
  - host: {{.PrivateIpAddress }}
  {{- end }}
{{ end  }}
