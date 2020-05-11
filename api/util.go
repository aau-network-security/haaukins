package api

const waitingHTMLTemplate = `
<html lang="en" dir="ltr">
		  <meta http-equiv="refresh" content="70" />
		  <head>
			<style>
				html, body {
		  height: 100%;
		  width: 100%;
		  margin: 0;
		  padding: 0;
		  font-size: 100%;
		  background: #191a1a;
		  text-align: center;
		}
		
		h1 {
		  margin: 100px;
		  padding: 0;
		  font-family: ‘Arial Narrow’, sans-serif;
		  font-weight: 100;
		  font-size: 1.1em;
		  color: #a3e1f0;
		}
		h2 {
		  margin:50px;
		  color: #a3e1f0;
		  font-family: ‘Arial Narrow’, sans-serif;
		}
		
		span {
		  position: relative;
		  top: 0.63em;  
		  display: inline-block;
		  text-transform: uppercase;  
		  opacity: 0;
		  transform: rotateX(-90deg);
		}
		
		.let1 {
		  animation: drop 1.2s ease-in-out infinite;
		  animation-delay: 1.2s;
		}
		
		.let2 {
		  animation: drop 1.2s ease-in-out infinite;
		  animation-delay: 1.3s;
		}
		
		.let3 {
		  animation: drop 1.2s ease-in-out infinite;
		  animation-delay: 1.4s;
		}
		
		.let4 {
		  animation: drop 1.2s ease-in-out infinite;
		  animation-delay: 1.5s;
		
		}
		
		.let5 {
		  animation: drop 1.2s ease-in-out infinite;
		  animation-delay: 1.6s;
		}
		
		.let6 {
		  animation: drop 1.2s ease-in-out infinite;
		  animation-delay: 1.7s;
		}
		
		.let7 {
		  animation: drop 1.2s ease-in-out infinite;
		  animation-delay: 1.8s;
		}
		
		@keyframes drop {
			10% {
				opacity: 0.5;
			}
			20% {
				opacity: 1;
				top: 3.78em;
				transform: rotateX(-360deg);
			}
			80% {
				opacity: 1;
				top: 3.78em;
				transform: rotateX(-360deg);
			}
			90% {
				opacity: 0.5;
			}
			100% {
				opacity: 0;
				top: 6.94em
			}
		}
    </style>
  </head>
  <body>
  <h1>
    <span class="let1">l</span>  
    <span class="let2">o</span>  
    <span class="let3">a</span>  
    <span class="let4">d</span>  
    <span class="let5">i</span>  
    <span class="let6">n</span>  
    <span class="let7">g</span>  
  </h1>
<h2>
Virtualized Environment
</h2>
  </body>
</html>
`

const errorHTMLTemplate = `
<html lang="en" dir="ltr">
  <body>
	bad request
  </body>
</html>
`

