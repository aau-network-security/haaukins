package ctfd

const (
	aauIndex = `<div class="row">
    <style>
     .col-container:after { content: ""; display: table; clear: both; }
     .col { float: left; }
    </style>
    <div class="col-md-6 offset-md-3">
	<h3 class="text-center" style="padding-top: 10vh;">
	    <p>National Training Platform</p>
	</h3>
	<p class="text-center">
	    A platform for Cyber Security Exercises 
	</p>
	<p class="text-center">
	    Founded by <a href="http://danishcybersecurityclusters.dk/">Danish Cyber Security Clusters</a>
	</p>
	<a href="http://danishcybersecurityclusters.dk/">
	    <img class="w-100 mx-auto d-block" style="max-width: 300px; padding: 3vh 0 4vh 0;" src="{{.Host}}/assets/img/dcsc_logo.png">
	</a>
	<p class="text-center">
	    <p class="text-center">
		Developed at <a href="http://es.aau.dk/">Aalborg University</a> (Department of Electronic Systems) by:
	    </p>
	    <div class="col-container" style="margin-top: 40px;">
		<div class="col" style="width: 40%">
	      <img src="{{.Host}}/assets/img/logo.png" style="margin-left: 40px; max-width: 120px;">
	    </div>
		<div class="col" style="width: 60%; font-size:14px;">
		    <p><a href="https://github.com/kdhageman">Kaspar Hageman</a> (Ph.D. Student)</p>
		    <p><a href="https://github.com/tpanum">Thomas Kobber Panum</a> (Ph.D. Student)</p>
		    <p><a href="https://github.com/eyJhb">Johan Hempel Bengtson</a> (Student Helper)</p>
	    </div>
	    </div>
	</p>
	<p class="text-center" style="margin-top: 40px">
	    Feel free to join our local Facebook Group:
	</p>
	<p class="text-center">
	    <a href="https://hack.aau.dk"><i class="fab fa-facebook" aria-hidden="true"></i>&nbsp;AAU Hackers &amp; Friends</a>
	</p>
    </div>
</div>`

	aauCss = `.bg-dark {
    background-color: #211a52 !important;
}

.btn-dark {
    background-color: #211a52;
    border-color: #211a52;
}

.jumbotron {
    background-color: #211a52;
}

select.form-control {
   height: inherit !important;
}`
)
