# Prometheus-Configmanager

Prometheus Configmanager consists of two HTTP-based configuration services for Prometheus and Alertmanager configurations. Both Prometheus and Alertmanager use yaml files configuration, and are only modifiable by directly rewriting these files and then sending a request to the respective service to reload the configuration files. Configmanager provides an HTTP API to modify and reload these configuration files remotely (alertmanager.yml and alert rules files used by prometheus).

## Multi-tenancy

Both configmanagers are built with multiple tenants in mind. API paths require a `tenant_id` which uniquely identifies a tenant using the system. While multiple tenants operate on the same configuration, there is no worry about conflict as every alerting rule, routing receiver, or other component is kept distinct by using the tenant ID.

The basic way of providing multitenancy in prometheus components is by using labels. For example, in a multitenant alertmanager-configurer setup, each alert is first routed on the tenancy label, and then the routing tree is distinct for each tenant. With prometheus, alerting rules can be restricted so that each rule can only be triggered by metrics which have the label `{tenancyLabel: tenant_id}`.

### Prometheus

Command line Arguments:
```
  -port string
        Port to listen for requests. Default is 9100 (default "9100")
  -prometheusURL string
        URL of the prometheus instance that is reading these rules. Default is prometheus:9090 (default "prometheus:9090")
  -multitenant-label string
        The label name to segment alerting rules to enable multi-tenant support, having each tenant's alerts in a separate file. Default is tenant (default "tenant")
  -restrict-queries
        If this flag is set all alert rule expressions will be restricted to only match series with {<multitenant-label>=<tenant>}
  -rules-dir string
        Directory to write rules files. Default is '.' (default ".")
```

### Alertmanager

Command line Arguments
```
  -alertmanager-conf string
        Path to alertmanager configuration file. Default is ./alertmanager.yml (default "./alertmanager.yml")
  -alertmanagerURL string
        URL of the alertmanager instance that is being used. Default is alertmanager:9093 (default "alertmanager:9093")
  -multitenant-label string
        LabelName to use for enabling multitenancy through route matching. Leave empty for single tenant use cases.
  -port string
        Port to listen for requests. Default is 9101 (default "9101")
```


## HTTP API Documentation

Swagger documentation for the APIs can be found at `prometheus/docs/swagger-v1.yml` and `alertmanager/docs/swagger-v1.yml`

## Operation

The general way of using these services is by letting them take control of your Prometheus and Alertmanager configuration files. As such, they should be run on the same pod (if using kubernetes) as those services. Once set up, it is best to not edit these files manually as you may put it in a bad state that configmanager is not able to understand. Note that prometheus.yml is not directly modified by these services, so that is safe so long as you have a section like below:

```
rule_files:
  - '/etc/prometheus/alert_rules/*_rules.yml'
```

Where at least one of the elements in the array is pointed to the same directory that configmanager is writing the rules files (controlled by command line arguments).


## Building Docker Containers

Use the following commands to build the containers:
```
docker build -t prometheus-configurer -f  Dockerfile.prometheus .
docker build -t alertmanager-configurer -f  Dockerfile.alertmanager .
```

## Third-Party Code Disclaimer
Prometheus-Configmanager contains dependencies which are not maintained by the maintainers of this project. Please read the disclaimer at THIRD_PARTY_CODE_DISCLAIMER.md.

## License

Prometheus-Configmanager is MIT License licensed, as found in the LICENSE file.
