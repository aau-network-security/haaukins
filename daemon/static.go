package daemon

import "strings"

const (
	notfoundpage = `
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>404 Not Found &mdash; Haaukins </title>
  <style>
			   
		//@import url('https://fonts.googleapis.com/css?family=Roboto+Mono');
		@import url('https://fonts.googleapis.com/css?family=Orbitron|Ubuntu&display=swap')
		// helper
		
		
		html, body {
		  font-family: 'Orbitron|Ubuntu', monospace;
		  font-size: 24px;
		}
		
		html {
		  box-sizing: border-box;
		  user-select: none;
		}
		
		body {
		  background-color: #211a52;
		}
		
		*, *:before, *:after {
		  box-sizing: inherit;
		}
		
		.container {
		  width: 100%;
		  margin-top: 200px;
		}
		
		.copy-container {
		  text-align: center;
		}
		
		p {
		  color: #fff;
		  font-size: 24px;
		  letter-spacing: .2px;
		  margin: 0;
		}
		
		#cb-replay {
		  fill: #666;
		  width: 20px;
		  margin: 15px;
		  right: 0;
		  bottom: 0;
		  position: absolute;
		  overflow: inherit;
		  cursor: pointer;
		
		  &:hover {
			fill: #888;
		  }
		}
		
		.body-center {
  		 		margin: auto;
  				width: 60%;
  				padding: 10px;
		}
		
		.center {
		  display: block;
		  margin-left: auto;
		  margin-right: auto;
		  margin-top: 100px;
		
		}
		
		.link-container {
		  text-align: center;
		}
		a.more-link {
		  text-transform: uppercase;
		  font-size: 13px;
			background-color: #fff;
			padding: 10px 15px;
			border-radius: 0;
			color: #000;
			display: inline-block;
			margin-right: 5px;
			margin-bottom: 5px;
			line-height: 1.5;
			text-decoration: none;
		  margin-top: 50px;
		  letter-spacing: 1px;
		}

  </style>
</head>
<body style="margin: 0">
<div class="container">
  <div class="copy-container body-center">
    <p>
      404 Not Found
    </p>
    <img src="https://raw.githubusercontent.com/aau-network-security/haaukins/master/.github/logo/white240px.png"  class="center">
    
    <div class="link-container">
  <a target="_blank" href="https://github.com/aau-network-security/haaukins" class="more-link">Visit to our Github Repository</a>
</div>
  </div>

</div>

<svg version="1.1" id="cb-replay" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px" y="0px"
	 viewBox="0 0 279.9 297.3" style="enable-background:new 0 0 279.9 297.3;" xml:space="preserve">
<g>
	<path d="M269.4,162.6c-2.7,66.5-55.6,120.1-121.8,123.9c-77,4.4-141.3-60-136.8-136.9C14.7,81.7,71,27.8,140,27.8
		c1.8,0,3.5,0,5.3,0.1c0.3,0,0.5,0.2,0.5,0.5v15c0,1.5,1.6,2.4,2.9,1.7l35.9-20.7c1.3-0.7,1.3-2.6,0-3.3L148.6,0.3
		c-1.3-0.7-2.9,0.2-2.9,1.7v15c0,0.3-0.2,0.5-0.5,0.5c-1.7-0.1-3.5-0.1-5.2-0.1C63.3,17.3,1,78.9,0,155.4
		C-1,233.8,63.4,298.3,141.9,297.3c74.6-1,135.1-60.2,138-134.3c0.1-3-2.3-5.4-5.3-5.4l0,0C271.8,157.6,269.5,159.8,269.4,162.6z"/>
</g>
</svg>

</body>
</html>`
)

var (
	suspendPage = strings.ReplaceAll(notfoundpage, "404 Not Found", "307 Event Temporary Suspended")
)
