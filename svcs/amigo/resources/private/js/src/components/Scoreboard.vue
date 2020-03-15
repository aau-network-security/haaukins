<template>
    <div class="table-responsive-lg">
        <table class="table table-striped">
            <thead class="thead-dark-custom">
                <tr>
                    <th class="text-center">#</th>
                    <th>Team</th>
                    <th>Score</th>
                    <th v-for="chal in challenges" v-bind:key="chal">{{chal}}</th>
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
            }
        },
        created: function() {
            let url = new URL('/scores', window.location.href);
            url.protocol = url.protocol.replace('http', 'ws');
            this.connectToWS(url.href);
        },
        methods: {
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
        border-color:#211A52;
        color:inherit;
    }
</style>