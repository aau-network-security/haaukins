<template>
    <table class="table centerbox is-striped">
        <thead>
        <tr>
            <th></th>
            <th>Team</th>
            <th v-for="chal in challenges" v-bind:key="chal.challenge.Tag"><abbr :title="chal.challenge.Name">{{ chal.challenge.Tag }}</abbr></th>
        </tr>
        </thead>
        <tbody>
        <team-row v-for="(team, index) in teams" v-bind:key="team.id" :name="team.name" :completions="team.completions" :pos="index + 1"></team-row>
        <tr>
            <td></td>
            <td>{{teams}}</td>
            <td>{{challenges}}</td>
        </tr>
        </tbody>
    </table>

</template>

<script>
    import TeamRow from './TeamRow.vue'
    /* eslint-disable */
    //ristrutturare completamente la scoreboard. anche i punti totali del team sono necessari
    //credo che una struttura json da ricevere sia la seguente
    // {
    //     chals:[
    //         "aaa", "bbb"
    //     ],
    //     teams: [
    //         {
    //            "name": name,
    //            points: points,
    //            "completed": [....]
    //         }
    //     ]
    // }
    //
    // l,array completed sara collegato a chals, ovvero l'index 0 corrispondera al index 0 della chals
    //per fare questo in amigo serve avere un doppio for in cui prima si scrollano le challenge e dentro i team

    export default {
        name: 'scoreboard',
        data: () => {
            return {
                teams: [],
                challenges: [],
            }
        },
        created: function() {
            var url = new URL('/scores', window.location.href);
            url.protocol = url.protocol.replace('http', 'ws');
            this.connectToWS(url.href);
        },
        methods: {
            connectToWS: function(url) {
                var self = this;
                var ws = new WebSocket(url);
                ws.onmessage = self.receiveMsg
                ws.onclose = function(){
                    ws = null;
                    setTimeout(function(){self.connectToWS(url)}, 3000);
                };
            },
            receiveMsg: function(evt) {
                var messages = evt.data.split('\n');
                window.console.log(evt)
                for (var i = 0; i < messages.length; i++) {
                    const msg = messages[i];
                    const json = JSON.parse(msg);
                    window.console.log(json.value)
                    switch (json.msg) {
                        case "challenges":
                            this.challenges = json.values;
                            break;
                        case "teams":
                            this.teams = json.values;
                            break;
                    }
                }
            },
        },
        components: {
            TeamRow,
        }
    }
</script>