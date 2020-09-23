<template>
    <div id="challenges-board">

      <b-button class="btn-haaukins" v-on:click="next()"></b-button>
      <b-carousel
          ref="stepsCarousel"
          :interval=0
          class="h-50"
      >
        <b-carousel-slide v-for="step in challengesFromAmigo" v-bind:key="step.number">
          <template slot="img">
            <div class="h-100 step-content" v-bind:class="{ 'step-': !step.is_solved}"> <!-- todo finish this (make the block screen in the step is not solved)-->
              <div class="row mt-5" v-for="category in sortChallenges(step.challenges)" v-bind:key="category[0].challenge.Category">
                <div class="category-header col-md-12 mb-3">
                  <h3>{{category[0].challenge.Category}}</h3>
                </div>
                <div class="col-lg-3 col-md-4" v-for="el in category" v-bind:key="el.challenge.Tag">
                  <button class="btn challenge-button w-100 text-truncate pt-3 pb-3 mb-2" v-on:click="openModal(el)" v-bind:class="{'btn-success': el.isUserCompleted, 'btn-haaukins': !el.isUserCompleted}">
                    <p class="chal-name-font">{{el.challenge.Name}}</p>
                    <span>{{el.challenge.Points}}</span>
                  </button>
                </div>
              </div>
            </div>
          </template>
        </b-carousel-slide>
      </b-carousel>
        <challenge-modal :challenge="this.chalInfo" :teamsCompleted="this.teamsCompleted" v-on:challengeCompleteReload="challengeCompleteReload"></challenge-modal>
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
                teamsCompleted: [], //passed to the modal
                challengesFromAmigo: [], //they keys are the categories, each category has a list of challenges
                currentStep: 0,
            }
        },
        created: function() {

            this.connectToWS();
        },
        methods: {
            next: function (){
              this.$refs.stepsCarousel.next()
            },
            sortChallenges: function(step_challenges){

              if (step_challenges === undefined) {
                return
              }

                let challenges = {};
                //Sort the challenges per category
                step_challenges.forEach(function (el) {
                    if (!(el.challenge.Category in challenges)){
                        challenges[el.challenge.Category] = []
                    }
                    challenges[el.challenge.Category].push(el)
                });


                //Sort the challenges for points
                for (let cat in challenges){
                    challenges[cat] = challenges[cat].sort((a, b) => a.challenge.Points - b.challenge.Points);
                }

                return challenges;
            },
            openModal: function (obj) {
                this.chalInfo = obj.challenge;
                this.teamsCompleted = obj.teamsCompleted;
                this.$bvModal.show('challengeModal')
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
                    if (json.msg === "steps"){
                        this.challengesFromAmigo = json.values;
                        window.console.log(json.values)
                    }
                }
                this.sortChallenges();
            },
            challengeCompleteReload: function () {
                this.connectToWS()
            }
        }
    }
</script>

<style>
    .carousel-inner, .carousel-item {
      height: 100%
    }
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