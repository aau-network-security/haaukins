<template>
    <div class="table-responsive">
        <table class="table table-striped ">
            <thead class="thead-dark-custom text-center">
                <tr>
                    <th colspan="3"></th>
                    <th v-for="chal in get_categories(challenges)" v-bind:colspan="get_challenges_categories(challenges, chal.category).length" class="scoreboard-border" v-bind:key="chal.category">{{chal.category}}</th>
                </tr>
                <tr>
                    <th class="text-center">#</th>
                    <th>Team</th>
                    <th>Score</th>
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
                <tr class="text-center"><td :colspan="challenges.length + 3">No team registered to this event!</td></tr>
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
                rows_color: ['#25308B', '#3A4496', '#25308B', '#3A4496', '#3A4496']
            }
        },
        created: function() {
            let url = new URL('/scores', window.location.href);
            url.protocol = url.protocol.replace('http', 'ws');
            this.connectToWS(url.href);
        },
        methods: {
            get_categories: function(full_categories){
                let categories = []
                for (let i in full_categories){
                    if (full_categories[i].chals.length > 0) {
                        full_categories[i].color = this.rows_color[i]
                        categories.push(full_categories[i])
                    }
                }
                window.console.log(categories)
                return categories
            },
            get_challenges: function(full_challenges){
                let challenges = []
                for (let i in full_challenges){
                    if (full_challenges[i].chals.length > 0) {
                        for (let j in full_challenges[i].chals){
                            challenges.push(full_challenges[i].chals[j])
                        }
                    }
                }
                return challenges
            },
            get_challenges_categories: function(full_challenges, chal_category){
                let challenges = []
                for (let i in full_challenges){
                    if (full_challenges[i].chals.length > 0) {
                        for (let j in full_challenges[i].chals){
                            if (full_challenges[i].category === chal_category) {
                                challenges.push(full_challenges[i].chals[j])
                            }
                        }
                    }
                }
                return challenges
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