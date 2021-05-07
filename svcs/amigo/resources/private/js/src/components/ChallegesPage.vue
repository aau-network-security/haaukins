<template>
  <div id="challenges-board" v-bind:style= "[this.isLabAssigned  ? {}: { 'pointer-events': 'none', 'opacity': '0.5' }]">

    <div class="row mt-2" v-for="category in challengesFromAmigo" v-bind:key="category[0].challenge.category">
      <div class="category-header col-md-12 mb-3">
        <h3>{{category[0].challenge.category}}</h3>
      </div>
      <div class="col-lg-3 col-md-4" v-for="el in category" v-bind:key="el.challenge.tag">
        <button class="btn challenge-button w-100 text-truncate pt-3 pb-3 mb-2" v-on:click="openModal(el)" v-bind:class="{'btn-disabled': el.isChalDisabled && !el.isUserCompleted, 'btn-haaukins': !el.isUserCompleted,'btn-success': el.isUserCompleted}">
          <p class="chal-name-font">{{ el.challenge.name }}</p>
          <span>{{ el.challenge.points }}</span>
        </button>
      </div>
    </div>

    <challenge-modal :challenge="this.chalInfo" :teamsCompleted="this.teamsCompleted" v-on:runChallenge="runChallenge"  v-on:resetChallenge="resetChallenge" v-on:challengeCompleteReload="challengeCompleteReload"></challenge-modal>
  </div>
</template>

<script>
import ChallengeModal from "./ChallengeModal";

export default {
  name: "ChallegesPage",
  components: {ChallengeModal},
  data: function () {
    return {
      chalInfo: {}, //passed to the modal
      isLabAssigned: false,
      teamsCompleted: [], //passed to the modal
      challengesFromAmigo: [], //they keys are the categories, each category has a list of challenges
    }
  },
  created: function() {

    this.connectToWS();
  },
  methods: {
    sortChallenges: function(){

      let challenges = {};

      //Sort the challenges per category
      this.challengesFromAmigo.forEach(function (el) {
        if (!(el.challenge.category in challenges)){
          challenges[el.challenge.category] = []
        }
        challenges[el.challenge.category].push(el)
      }, this);


      //Sort the challenges for points
      for (let cat in challenges){
        challenges[cat] = challenges[cat].sort((a, b) => a.challenge.points - b.challenge.points);
      }

      this.challengesFromAmigo = challenges;
    },
    openModal: function (obj) {
      this.chalInfo = obj.challenge;
      this.teamsCompleted = obj.teamsCompleted;
      if (this.isLabAssigned) {
        this.$bvModal.show('challengeModal')
      }
    },
    connectToWS: function() {
      let url = new URL('/challengesFrontend', window.location.href);
      url.protocol = url.protocol.replace('http', 'ws');
      let self = this;
      let ws = new WebSocket(url);
      ws.onmessage = self.receiveMsg;
      ws.onclose = function(){
        ws = null;
        setTimeout(function(){self.connectToWS(url)}, 3000);
      };
    },
    receiveMsg: function(evt) {
      let messages = evt.data.split('\n');
      for (let i = 0; i < messages.length; i++) {
        let msg = messages[i];
        let json = JSON.parse(msg);
        if (json.msg === "challenges"){
          this.challengesFromAmigo = json.values;
          this.isLabAssigned = json.isLabAssigned
        }
      }
      this.sortChallenges();
    },
    challengeCompleteReload: function () {
      this.connectToWS()
    },
    runChallenge: function () {
      this.connectToWS()
    },
    resetChallenge: function () {
      this.connectToWS()
    }
  }
}
</script>

<style>
.btn-haaukins{
  color: #fff;
  background-color: #211A52;
  border-color: #211A52;
}
.btn-haaukins:hover{
  color: #fff;
  background-color: #1a1441;
  border-color: #1a1441;
}

.btn-disabled{
  color: #fff;
  background-color: #949494;
  border-color: #565667;
}

.btn-disabled:hover{
  color: #fff;
  background-color: #6c6499;
  border-color: #1a1441;
}
.btn-success{
  background-color: #6ab55f;
  border-color: #6ab55f;
}
.btn-success:hover{
  background-color: #55a04a;
  border-color: #55a04a;
}
.chal-name-font{
  font-size: 14px;
}
</style>