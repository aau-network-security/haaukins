<template>
  <div class="row">
    <div class="col-sm-12 col-lg-8 offset-lg-2">
      <h1 class="text-center mt-toppage glow">Haaukins</h1>
      <h4 class="text-center glow"> Cyber Security Training Platform</h4>
      <p class="text-center mt-4" style="font-size: 1.25em">
        Fonded by: <a class="color-sec-aau" href="https://www.industriensfond.dk/"> Industriens Fond </a>
      </p>


      <div class="col text-center"><a href="https://www.industriensfond.dk/">
        <img v-if="$theme == 'light'" style="max-width: 500px" src="/assets/img/if_logo_black.png">
        <img v-if="$theme == 'dark'" style="max-width: 500px" src="/assets/img/if_logo_white.png">
      </a> </div>
      <p class="text-center mt-4" style="font-size: 1.25em">
        Promoted by: <a class="color-sec-aau" href="https://www.cyberskills.dk/"> CyberSkills </a>
      </p>
      <div class="col text-center"><a href="https://www.cyberskills.dk/">
        <img v-if="$theme == 'light'" style="max-width: 500px;" src="/assets/img/cyberskills-community.png">
        <img v-if="$theme == 'dark'" style="max-width: 500px;" src="/assets/img/cyberskills_logo_white.png">
      </a> </div>

      <p class="text-center mt-4" style="font-size: 1.25em" >
        Developed at <a class="color-sec-aau" href="https://www.es.aau.dk/">Aalborg University</a> by:
      </p>
      <div class="row aau-developer mt-5">
        <div class="dev-container row" style="margin: auto; width: 600px;">
          <div class="col text-center">
            <img v-if="$theme == 'light'" src="/assets/img/haaukins-login.png" class="spin">
            <img v-if="$theme == 'dark'" src="/assets/img/haaukins-login_white.png" class="spin">
          </div>
          <div class="col mt-3 mt-sm-4" style="margin-left: 50px;">
            <p><a class="color-sec-aau" href="https://mrturkmen.com" target="_blank">Ahmet Turkmen</a></p>
            <p><a class="color-sec-aau" href="https://www.linkedin.com/in/mikkelhost/" target="_blank">Mikkel Høst Christiansen</a></p>
            <p><a class="color-sec-aau" href="https://github.com/gianmarcomennecozzi" target="_blank">Gian Marco Mennecozzi</a></p>
          </div>
        </div>
      </div>
      <p class="text-center mt-4" style="font-size: 1.25em">
        Founded by:
      </p>
      <div class="row aau-developer text-center">
        <div class="col"><p><a href="https://www.linkedin.com/in/jensmyrup/" target="_blank">Jens Myrup Pedersen</a></p></div>
        <div class="col"><p><a href="https://github.com/tpanum" target="_blank">Thomas Kobber Panum</a></p></div>
        <div class="col"><p><a href="https://github.com/kdhageman" target="_blank">Kaspar <br> Hageman</a></p></div>
        <div class="col"><p><a href="https://github.com/eyJhb" target="_blank">Johan Hempel Bengtson</a></p></div>
      </div>
      <p class="text-center mt-4 card-title">
        Publications
      </p>
      <div class="row aau-developer text-center">
        <div class="col"><p><a href="https://ieeexplore.ieee.org/document/8820918" target="_blank">Haaukins: A Highly Accessible and Automated Virtualization Platform for Security Education</a> </p></div>
        <div class="col"><p><a href="https://ieeexplore.ieee.org/document/9453985" target="_blank">Bridging the Gap: Adapting a Security Education Platform to a New Audience</a> </p></div>
      </div>


      <div class="card-deck py-4">
        <div class="card text-center">
          <div class="card-body">
            <h5 class="card-title">Tips and tricks</h5>
            <div class="card-text">
              Stuck at a certain challenge? Or do you just want to know more about a certain topic?
            </div>
          </div>
          <div class="card-footer">
            <a href="https://aau-network-security.github.io/tips-and-tricks/" target="_blank">Visit the tips &amp; tricks page</a>
          </div>
        </div>
        <div class="card text-center">
          <div class="card-body">
            <h5 class="card-title">Survey</h5>
            <p>You can help us improve the platform by taking our survey to let us know about your experiences!</p>
          </div>
          <div class="card-footer">
            <a href="https://www.survey-xact.dk/LinkCollector?key=KDRVSTDJJN15" target="_blank">Fill out the survey here</a>
          </div>
        </div>
      </div>
      <p class="text-center">
        Feel free to join our local Facebook Group:
      </p>
      <p class="text-center facebook-aau">
        <a href="https://www.facebook.com/groups/957517617737780"><i class="fab fa-facebook" aria-hidden="true"></i>&nbsp;AAU Hackers &amp; Friends</a>
      </p>
      <ul class="text-center mb-botpage sponsor">
        <li><a href="https://eadania.dk/"> <img src="/assets/img/index/da-90.png"></a></li>
        <li><a href="https://www.dtu.dk/"><img src="/assets/img/index/dtu-90.png"></a></li>
        <li><a href="https://kea.dk/"> <img src="/assets/img/index/
					kea-90.jpg"></a></li>
        <li v-if="$theme == 'light'"><a href="https://happy42.dk/"> <img src="/assets/img/index/happy-90.png"></a></li>
        <li v-if="$theme == 'dark'"><a href="https://happy42.dk/"> <img src="/assets/img/index/happy-90_white.png"></a></li>
        <li><a href="https://www.eaaa.dk/"><img src="/assets/img/index/eaa-90.png"></a></li>
        <li><a href="https://www.cbs.dk/"><img  src="/assets/img/index/cbs.png"></a></li>
      </ul>
    </div>
  </div>
</template>

<script>
export default {
  name: "IndexPage"
}
</script>

<style scoped>

</style>