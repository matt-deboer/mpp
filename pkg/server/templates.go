package main

import (
	"time"

	"github.com/matt-deboer/mpp/pkg/router"
)

type templateData struct {
	Uptime       time.Duration
	RouterStatus *router.Status
	Version      string
	GoVersion    string
}

var clusterStatusTemplate = `
<!DOCTYPE html>
<html lang="en">
	<head>
		<meta http-equiv="Content-Type" content="text/html; charset=utf-8">
		<title>Multi-Prometheus Proxy Cluster Info</title>
		<!--<link rel="shortcut icon" href="/static/img/favicon.ico">-->
		<!-- Latest compiled and minified CSS -->
		<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.7/css/bootstrap.min.css" integrity="sha384-BVYiiSIFeK1dGmJRAkycuHAHRg32OmUcww7on3RYdg4Va+PmSTsz/K68vbdEjh4u" crossorigin="anonymous">

		<!-- Optional theme -->
		<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.7/css/bootstrap-theme.min.css" integrity="sha384-rHyoN1iRsVXV4nD0JutlnGaslCJuC7uwjduW9SVrLvRYooPp2bWYgmgJQIXwl/Sp" crossorigin="anonymous">

		<script src="https://code.jquery.com/jquery-3.2.1.min.js"></script>

		<!-- Latest compiled and minified JavaScript -->
		<script src="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.7/js/bootstrap.min.js" integrity="sha384-Tc5IQib027qvyjSMfHjOMaLkfuWVxZxUPnCJA7l2mCWNIpG9mGCD8wGNIcPD7Txa" crossorigin="anonymous"></script>
		
		<script>
			var PATH_PREFIX = "";
			$(function () {
				$('[data-toggle="tooltip"]').tooltip()
			})
		</script>

		<style>
		body {
			padding-top: 50px;
			padding-bottom: 20px;
		}

		th.job_header {
			font-size: 20px;
			padding-top: 10px;
			padding-bottom: 10px;
		}

		.state_indicator {
			padding: 0 4px 0 4px;
		}

		.literal_output td {
			font-family: monospace;
		}

		.cursor-pointer {
			cursor: pointer;
		}

		.tooltip-inner {
			max-width: none;
			text-align: left;
		}
		</style>
	</head>

	<body>
		<nav class="navbar navbar-inverse navbar-fixed-top">
			<div class="container-fluid">
				<div class="navbar-header">
					<button type="button" class="navbar-toggle collapsed" data-toggle="collapse" data-target="#navbar" aria-expanded="false" aria-controls="navbar">
						<span class="sr-only">Toggle navigation</span>
						<span class="icon-bar"></span>
						<span class="icon-bar"></span>
						<span class="icon-bar"></span>
					</button>
					<a class="navbar-brand" href="/">MPP<span class="glyphicon glyphicon-fire" aria-hidden="true"></a>
				</div>
				<div id="navbar" class="navbar-collapse collapse">
					<ul class="nav navbar-nav navbar-left">
						
						
						<!--<li><a href="/alerts">Alerts</a></li>
						<li><a href="/graph">Graph</a></li>
						<li class="dropdown">
							<a href="#" class="dropdown-toggle" data-toggle="dropdown" role="button" aria-haspopup="true" aria-expanded="false">Status <span class="caret"></span></a>
							<ul class="dropdown-menu">
								<li><a href="/status">Runtime &amp; Build Information</a></li>
								<li><a href="/flags">Command-Line Flags</a></li>
								<li><a href="/config">Configuration</a></li>
								<li><a href="/rules">Rules</a></li>
								<li><a href="/targets">Targets</a></li>
							</ul>
						</li>
						<li>
							<a href="https://prometheus.io" target="_blank">Help</a>
						</li>-->
					</ul>
				</div>
			</div>
		</nav>

	<div class="container-fluid">
		<h2 id="runtime">Runtime Information</h2>
    <table class="table table-condensed table-bordered table-striped table-hover">
      <tbody>
        <tr>
          <th>Uptime</th>
          <td>{{.Uptime}}</td>
        </tr>
        <tr>
          <th>Selector Strategy</th>
          <td><code>{{.RouterStatus.Strategy}}</code></td>
        </tr>
				<tr>
          <th>Comparison Metric</th>
          <td><code>{{.RouterStatus.ComparisonMetric}}</code></td>
        </tr>
				<tr>
          <th>Selection Interval</th>
          <td>{{.RouterStatus.Interval}}</td>
        </tr>
      </tbody>
    </table>

    <h2 id="buildinformation">Build Information</h2>
    <table class="table table-condensed table-bordered table-striped table-hover">
      <tbody>
        <tr>
          <th scope="row">Version</th>
          <td>{{.Version}}</td>
        </tr>
        <tr>
          <th scope="row">GoVersion</th>
          <td>{{.GoVersion}}</td>
        </tr>
      </tbody>
    </table>
		
		<h2 id="runtime">Prometheus Endpoints</h2>
		<table class="table table-condensed table-bordered table-striped table-hover">
			<tbody>
				<tr>
					<th>Endpoint</th>
					<th>Selected</th>
					<th>Uptime</th>
					<th><code>{{.RouterStatus.ComparisonMetric}}</code></th>
				</tr>
				{{block "list" .RouterStatus.Endpoints}}{{range .}}
				<tr>
					<td><a href="{{.Address}}/status">{{.Address}}</a></td>
					<td>{{if .Selected}}<span class="glyphicon glyphicon-check" aria-hidden="true"></span>{{end}}</td>
					<td>{{if .Uptime}}{{.Uptime}}{{else}}<span class="glyphicon glyphicon-remove" aria-hidden="true"></span><em>&nbsp; unavailable</em>{{end}}</td>
					<td>{{.ComparisonMetricValue}}</td>
				</tr>
				{{end}}{{end}}
			</tbody>
		</table>

	</div>

	</body>
</html>
`
