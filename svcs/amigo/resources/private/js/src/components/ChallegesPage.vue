<template>
    <div id="challenges-board">
        <div class="row mt-5" v-for="category in challengesFromAmigo" v-bind:key="category[0].challenge.Category">
            <div class="category-header col-md-12 mb-3">
                <h3>{{category[0].challenge.Category}}</h3>
            </div>
            <div class="col-lg-3 col-md-4" v-for="el in category" v-bind:key="el.challenge.Tag">
                <button class="btn challenge-button w-100 text-truncate pt-3 pb-3 mb-2" v-on:click="openModal(el)" v-bind:class="{'btn-success': el.isUserCompleted, 'btn-haaukins': !el.isUserCompleted}">
                    <p>{{el.challenge.Name}}</p>
                    <span>{{el.challenge.Points}}</span>
                </button>
            </div>
        </div>
        <challenge-modal :challenge="this.chalInfo" :teamsCompleted="this.teamsCompleted"></challenge-modal>
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
                dummyChallenges: {
                    "challenges": [
                        { "challenge": { "Tag": "aaa", "Name": "FTP dwa Login", "EnvVar": "APP_FLAG", "Static": "", "Points": 17, "Description": "Find a way to get flag from FTP server. (Go john!)", "Category": "Forensics" }, "isUserCompleted": false, "teamsCompleted": [ { "teamName": "menne", "completedAt": "2020-03-13T15:20:14.198192946+01:00" } ] },
                        { "challenge": { "Tag": "aaawww", "Name": "FTP dwa Login", "EnvVar": "APP_FLAG", "Static": "", "Points": 1, "Description": "Find a way to get flag from FTP server. (Go john!)", "Category": "Forensics" }, "isUserCompleted": false, "teamsCompleted": [ { "teamName": "menne", "completedAt": "2020-03-13T15:20:14.198192946+01:00" } ] },
                        { "challenge": { "Tag": "ww", "Name": "FTP dwa Login", "EnvVar": "APP_FLAG", "Static": "", "Points": 1, "Description": "Find a way to get flag from FTP server. (Go john!)", "Category": "Forensics" }, "isUserCompleted": false, "teamsCompleted": [ { "teamName": "menne", "completedAt": "2020-03-13T15:20:14.198192946+01:00" } ] },
                        { "challenge": { "Tag": "aa", "Name": "FTP dwa Login", "EnvVar": "APP_FLAG", "Static": "", "Points": 1, "Description": "Find a way to get flag from FTP server. (Go john!)", "Category": "Forensics" }, "isUserCompleted": false, "teamsCompleted": [ { "teamName": "menne", "completedAt": "2020-03-13T15:20:14.198192946+01:00" } ] },
                        { "challenge": { "Tag": "xssww-1", "Name": "Cross-site dwaww", "EnvVar": "APP_FLAG", "Static": "", "Points": 5, "Description": "This exercise consists of two mahines; a webserver and a client. The names says the rest.", "Category": "Web exploitation" }, "isUserCompleted": false, "teamsCompleted": [ { "teamName": "teset", "completedAt": "2020-03-13T15:28:33.61593751+01:00" } ] },
                        { "challenge": { "Tag": "ww-1", "Name": "Cross-site dwaww", "EnvVar": "APP_FLAG", "Static": "", "Points": 5, "Description": "This exercise consists of two mahines; a webserver and a client. The names says the rest.", "Category": "Web exploitation" }, "isUserCompleted": false, "teamsCompleted": [ { "teamName": "teset", "completedAt": "2020-03-13T15:28:33.61593751+01:00" } ] },
                        { "challenge": { "Tag": "aa-1", "Name": "Cross-site dwaww", "EnvVar": "APP_FLAG", "Static": "", "Points": 5, "Description": "This exercise consists of two mahines; a webserver and a client. The names says the rest.", "Category": "Web exploitation" }, "isUserCompleted": false, "teamsCompleted": [ { "teamName": "teset", "completedAt": "2020-03-13T15:28:33.61593751+01:00" } ] },
                    ]
                }
            }
        },
        created: function() {
            let url = new URL('/challengesFrontend', window.location.href);
            url.protocol = url.protocol.replace('http', 'ws');
            this.connectToWS(url.href);
        },
        methods: {
            sortChallenges: function(){

                let challenges = {};

                //Sort the challenges per category
                this.challengesFromAmigo.forEach(function (el) {
                    if (!(el.challenge.Category in challenges)){
                        challenges[el.challenge.Category] = []
                    }
                    challenges[el.challenge.Category].push(el)
                }, this);

                //todo delete this
                this.dummyChallenges.challenges.forEach(function (el) {
                    if (!(el.challenge.Category in challenges)){
                        challenges[el.challenge.Category] = []
                    }
                    challenges[el.challenge.Category].push(el)
                }, this);

                //Sort the challenges for points
                for (let cat in challenges){
                    challenges[cat] = challenges[cat].sort((a, b) => a.challenge.Points - b.challenge.Points);
                }

                this.challengesFromAmigo = challenges;
            },
            openModal: function (obj) {
                window.console.log(obj);
                this.chalInfo = obj.challenge;
                this.teamsCompleted = obj.teamsCompleted;
                this.$bvModal.show('challengeModal')
            },
            connectToWS: function(url) {
                let self = this;
                let ws = new WebSocket(url);
                ws.onmessage = self.receiveMsg
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
                    if (json.msg == "challenges"){
                        this.challengesFromAmigo = json.values;
                    }
                }
                this.sortChallenges();
            },
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
    .btn-success{
        background-color: #6ab55f;
        border-color: #6ab55f;
    }
    .btn-success:hover{
        background-color: #55a04a;
        border-color: #55a04a;
    }
</style>