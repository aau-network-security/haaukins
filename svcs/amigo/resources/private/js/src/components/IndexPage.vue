<template>
  <div class="row">
    <div class="col-sm-12 col-lg-8 offset-lg-2">
      <h1 class="text-center mt-toppage glow"> Haaukins </h1>

      <h4 class="text-center glow"> Cyber Security Training Platform</h4>
      <p class="text-center mt-4 ubuntu-font" >
        Fonded by: <a class="color-sec-aau" href="https://www.industriensfond.dk/"> Industriens Fond </a>
      </p>

      <div class="col text-center"><a href="https://www.industriensfond.dk/">
        <img  id='if_logo' style="max-width: 500px" v-bind:src="'/assets/img/' +if_logo">
      </a> </div>
      <p class="text-center mt-4 ubuntu-font" >
        Promoted by: <a class="color-sec-aau" href="https://www.cyberskills.dk/"> CyberSkills </a>
      </p>
      <div class="col text-center"><a href="https://www.cyberskills.dk/">
        <img   v-bind:src="'/assets/img/' +cyberskills_logo" style="max-width: 500px;" >
      </a> </div>

      <p class="text-center mt-4 ubuntu-font" >
        Developed at <a class="color-sec-aau" href="https://www.es.aau.dk/">Aalborg University</a> by:
      </p>
      <div class="row aau-developer mt-5">
        <div class="dev-container row" style="margin: auto; width: 600px;">
          <div class="col text-center">
            <img id='haaukins_logo' class="spin" v-bind:src="'/assets/img/' +haaukins_logo">
          </div>
          <div class="col mt-3 mt-sm-4 ubuntu-font-small" >
            <p><a class="color-sec-aau" href="https://mrturkmen.com" target="_blank">Ahmet Turkmen</a></p>
            <p><a class="color-sec-aau" href="https://www.linkedin.com/in/mikkelhost/" target="_blank">Mikkel Høst Christiansen</a></p>
            <p><a class="color-sec-aau" href="https://github.com/gianmarcomennecozzi" target="_blank">Gian Marco Mennecozzi</a></p>
          </div>
        </div>
      </div>
      <p class="text-center mt-4 ubuntu-font">
        Founded by:
      </p>
      <div class="row aau-developer text-center ubuntu-font-small">
        <div class="col"><p><a href="https://www.linkedin.com/in/jensmyrup/" target="_blank">Jens Myrup Pedersen</a></p></div>
        <div class="col"><p><a href="https://github.com/tpanum" target="_blank">Thomas Kobber Panum</a></p></div>
        <div class="col"><p><a href="https://github.com/kdhageman" target="_blank">Kaspar Hageman</a></p></div>
        <div class="col"><p><a href="https://github.com/eyJhb" target="_blank">Johan Hempel Bengtson</a></p></div>
      </div>
      <p class="text-center mt-4 card-title">
        Publications
      </p>
      <div class="row aau-developer text-center ubuntu-font-small">
        <div class="col"><p><a href="https://ieeexplore.ieee.org/document/8820918" target="_blank">Haaukins: A Highly Accessible and Automated Virtualization Platform for Security Education</a> </p></div>
        <div class="col"><p><a href="https://ieeexplore.ieee.org/document/9453985" target="_blank">Bridging the Gap: Adapting a Security Education Platform to a New Audience</a> </p></div>
      </div>


      <div class="card-deck py-4">
        <div class="card text-center">
          <div class="card-body">
            <h5 class="card-title">Tips and tricks</h5>
            <div class="card-text ubuntu-font-small">
              Stuck at a certain challenge? Or do you just want to know more about a certain topic?
            </div>
          </div>
          <div class="card-footer ubuntu-font-small">
            <a style="color: #00e6cc" href="https://aau-network-security.github.io/tips-and-tricks/" target="_blank">Visit the tips &amp; tricks page</a>
          </div>
        </div>
        <div class="card text-center">
          <div class="card-body">
            <h5 class="card-title">Survey</h5>
            <p class="ubuntu-font-small">You can help us improve the platform by taking our survey to let us know about your experiences!</p>
          </div>
          <div class="card-footer ubuntu-font-small">
            <a style="color: #00e6cc" href="https://www.survey-xact.dk/LinkCollector?key=KDRVSTDJJN15" target="_blank">Fill out the survey here</a>
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
        <li><a href="https://kea.dk/"> <img src="/assets/img/index/kea-90.jpg"></a></li>
        <li><a href="https://happy42.dk/"> <img  id='happy_logo' v-bind:src="'/assets/img/index/' +happy_logo"></a></li>
        <li><a href="https://www.eaaa.dk/"><img src="/assets/img/index/eaa-90.png"></a></li>
        <li><a href="https://www.cbs.dk/"><img  src="/assets/img/index/cbs.png"></a></li>
        <div class="mt-5">
          <a  href="https://www.en.aau.dk/education/master/cyber-security/"><img  id='aau_logo' v-bind:src="'/assets/img/index/'+ aau_logo "></a>
        </div>
      </ul>
    </div>

  </div>
</template>

<script>


export default {
  name: "IndexPage",
  data: function(){
    return {
      haaukins_logo: 'haaukins-login.png',
      cyberskills_logo: 'cyberskills-community.png',
      if_logo: 'if_logo_black.png',
      happy_logo: 'happy-90.png',
      aau_logo : 'aau_dark_logo_left.png'
    }
  },
  mounted() {
    this.setImages()
  },

  methods: {
    setImages : function() {
      window.console.log("SET IMAGES CALLED !! ")
      if (this.$theme === 'dark') {
        this.haaukins_logo = 'haaukins-logo_white.png'
        this.aau_logo = 'aau_white_logo_left.png'
        this.happy_logo = 'happy-90_white.png'
        this.if_logo = 'if_logo_white.png'
        this.cyberskills_logo = 'cyberskills_logo_white.png'
      }else {
        this.haaukins_logo ='haaukins-login.png'
        this.aau_logo = 'aau_dark_logo_left.png'
        this.happy_logo = 'happy-90.png'
        this.if_logo = 'if_logo_black.png'
        this.cyberskills_logo = 'cyberskills-community.png'
      }
    }

  }

}
</script>

<style scoped>
  .card-footer .a {
    color: #00e6cc;
  }
  .custom-switch-button {
    border-radius: 2rem;
    background: white;
    color: #000000;
    border-color: white;
  }
 .switch-checkbox {
   display: none;
 }

.ubuntu-font {
  font-family: 'UbuntuMono-Regular';
  font-size: 1.25em;
}

.ubuntu-font-small {
  font-family: 'UbuntuMono-Regular';
  font-size: 0.9em;
}
</style>