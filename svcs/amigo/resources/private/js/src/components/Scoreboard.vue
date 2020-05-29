<template>
    <div class="table-responsive">
        <table class="table table-striped" id="scoreboardtable">
            <thead class="thead-dark-custom text-center">
                <tr>
                    <th class="text-center rank-col">#</th>
                    <th class="team-col">Team</th>
                    <th class="score-col">Score</th>
                    <th v-for="c in challenges" v-bind:colspan="c.chals.length" class="scoreboard-border" v-bind:key="c.category" v-bind:id="c.category">
                        {{category_name(c.category, c.chals.length)}}
                        <b-tooltip v-bind:target="c.category" triggers="hover" placement="top">
                            {{c.category}}
                        </b-tooltip>
                    </th>
                </tr>
                <tr>
                    <th class="rank-col"></th>
                    <th class="team-col"></th>
                    <th class="score-col"></th>
                    <th v-for="chal in get_challenges(challenges)" v-bind:key="chal.name" v-bind:id="chal.name" class="scoreboard-border">
                        <span class="chal-points-font">{{chal.points}}</span>
                        <b-tooltip v-bind:target="chal.name" triggers="hover" placement="bottom">
                            {{chal.name}}
                        </b-tooltip>
                    </th>
                </tr>
            </thead>
            <tbody v-if="teams.length > 0">
                <team-row v-for="(team, index) in teams" v-bind:key="team.id" :team="team" :pos="index + 1"></team-row>
            </tbody>
            <tbody v-else>
                <tr class="text-center"><td :colspan="get_challenges(challenges).length+ 3">No team registered to this event!</td></tr>
            </tbody>
        </table>

    </div>
</template>

<script>
    import TeamRow from './TeamRow.vue'

    export default {
        name: 'scoreboard',
        data: () => {
            return {
                teams: [],
                challenges: [],
            }
        },
        created: function() {
            let url = new URL('/scores', window.location.href);
            url.protocol = url.protocol.replace('http', 'ws');
            this.connectToWS(url.href);
        },
        methods: {
            get_challenges: function(full_challenges){
                let challenges = []
                for (let i in full_challenges){
                    for (let j in full_challenges[i].chals){
                        challenges.push(full_challenges[i].chals[j])
                    }
                }
                return challenges
            },
            category_name: function(category_name, challenges_num){
                if(challenges_num > 3){
                    return category_name
                }
                switch (category_name) {
                    case "Web exploitation":
                        return "Web E."
                    case "Forensics":
                        return "For.."
                    case "Cryptography":
                        return "Cry.."
                    case "Binary":
                        return "Bin.."
                    case "Reverse Engineering":
                        return "R. Eng."
                }
            },
            connectToWS: function(url) {
                let self = this;
                let ws = new WebSocket(url);
                ws.onmessage = self.receiveMsg;
                ws.onclose = function(){
                    ws = null;
                    setTimeout(function(){
                        self.connectToWS(url)
                    }, 3000);
                };
            },
            receiveMsg: function(evt) {
                let messages = evt.data.split('\n');
                for (let i = 0; i < messages.length; i++) {
                    const msg = messages[i];
                    const json = JSON.parse(msg);
                    if (json.msg == "scoreboard"){
                        this.challenges = json.values.challenges;
                        this.teams = json.values.teams.sort((a, b)=> b.tpoints - a.tpoints);
                    }
                }
            },

        },
        components: {
            TeamRow,
        }
    }
</script>

<style>
    table#scoreboardtable {
        table-layout: fixed!important;
        min-width: 1800px!important;
    }

    .table .thead-dark-custom th{
        color:#fff!important;
        background-color:#211A52;
        border-bottom: none;
        color:inherit;
    }
    .chal-points-font{
        font-family: 'Orbitron', sans-serif !important;
        letter-spacing: 1px;
    }
</style>