<template>
    <div id="challenges-board">
      <div id="stepProgressBar" class="text-center mt-4 mb-5">
        <div class="step" v-for="(step, index) in challengesFromAmigo" v-bind:key="step.number" >
          <div class="bullet" v-on:click="$refs.stepsCarousel.setSlide(index)" v-bind:class="{ 'completed': step.is_solved}">{{index + 1}}</div>
        </div>
      </div>
      <b-carousel ref="stepsCarousel" :interval=0>
        <b-carousel-slide v-for="step in challengesFromAmigo" v-bind:key="step.number" class="h-100">
          <template slot="img" class="h-100">
            <div class="step-content">
              <div v-bind:class="{ 'step-overlay': !step.is_solved}"></div>
              <div class="row" v-for="category in sortChallenges(step.challenges)" v-bind:key="category[0].challenge.Category">
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
                currentStep: 1,
            }
        },
        created: function() {
            this.connectToWS();
        },
        methods: {
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
                    }
                }
            },
            challengeCompleteReload: function () {
                this.connectToWS()
            }
        }
    }
</script>

<style>
    #stepProgressBar  {
        display:  flex;
        align-items: center;
        justify-content: center;
        overflow: auto;
    }

    .step  {
        text-align:  center;
        margin-left: 60px;
    }

    .step:nth-child(1)   {
        margin-left: 0px !important;
    }

    .bullet {
        border: 3px solid #211A52;
        height: 40px;
        width: 40px;
        border-radius: 100%;
        color: #211A52;
        display: inline-block;
        position: relative;
        transition: background-color 500ms;
        line-height:35px;
        font-family: 'Audiowide', cursive;
        font-size: 20px;
        cursor: pointer;
    }

    .bullet.completed  {
        color:  white;
        background-color:  #211A52;
    }

    .bullet::after {
        content: '';
        position: absolute;
        right: -64px;
        bottom: 16px;
        height: 5px;
        width: 64px;
        background-color: #211A52;
    }
    .step:last-child .bullet::after {
      width: 0px;
    }

    .h-100{
      height: 100%;
      min-height: 750px;
    }

    .step-overlay{
      position: absolute;
      top: 0;
      left: 0;
      height: 100%;
      width: 100%;
      z-index: 1;
      background-color: rgba(255, 255, 255, 0.5);
      border: solid 1px rgba(255, 255, 255, 0.5);
      border-radius: 10px;
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