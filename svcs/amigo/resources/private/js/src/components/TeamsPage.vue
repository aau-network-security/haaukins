<template>
    <div id="teams">
        <Plotly :data="traces" :layout="layout"></Plotly>
        <table class="table table-striped mt-5">
            <thead class="thead-dark-custom">
                <tr>
                    <th class="text-center">#</th>
                    <th>Team</th>
                    <th>Score</th>
                </tr>
            </thead>
            <tbody v-if="teams.length > 0">
                <tr v-for="(team, index) in teams" v-bind:key="team.id">
                    <td class="text-center">{{index + 1}}</td>
                    <td>{{team.name}}</td>
                    <td>{{team.tpoints}}</td>
                </tr>
            </tbody>
            <tbody v-else>
                <tr class="text-center"><td colspan="3">No team registered to this event!</td></tr>
            </tbody>
        </table>
    </div>
</template>

<script>
    import { Plotly } from 'vue-plotly'

    export default {
        name: "ScoreGraph",
        components: {
            Plotly
        },
        data: function () {
            return {
                teams: [],
                data: [{
                    x: [1, 2, 3, 4],
                    y: [10, 15, 13, 17],
                    type: "scatter"
                }],
                traces: [],
                layout: {
                    // title: 'Score Graph',
                    paper_bgcolor: 'rgba(0,0,0,0)',
                    plot_bgcolor: 'rgba(0,0,0,0)',
                    hovermode: 'closest',
                    xaxis: {
                        showgrid: false,
                        showspikes: true,
                    },
                    yaxis: {
                        showgrid: false,
                        showspikes: true,
                    },
                    legend: {
                        "orientation": "h"
                    }
                }
            }
        },
        created() {
            let url = new URL('/challengesFrontend', window.location.href);
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
                    setTimeout(function(){self.connectToWS(url)}, 3000);
                };
            },
            receiveMsg: function(evt) {
                let messages = evt.data.split('\n');
                for (let i = 0; i < messages.length; i++) {
                    let msg = messages[i];
                    let json = JSON.parse(msg);
                    if (json.msg === "scoreboard"){
                        this.teams = json.values.teams.sort((a, b)=> b.tpoints - a.tpoints);
                        this.sortSolvedData()
                    }
                }
            },
            sortSolvedData: function(){
                for (let i=0; i < this.teams.length; i++){

                    let list = [];
                    for (let j = 0; j < this.teams[i].completions.length; j++)
                        list.push({'date': this.teams[i].completions[j], 'points': this.teams[i].points[j]});

                    list.sort((a, b) => new Date(a.date) - new Date(b.date) )

                    for (let k = 0; k < list.length; k++) {
                        this.teams[i].completions[k] = list[k].date;
                        this.teams[i].points[k] = list[k].points;
                    }
                }
                this.scoreGraph()
            },
            scoreGraph: function () {

                for (let i=0; i < this.teams.length; i++){
                    let team_score = [];
                    let times = [];
                    for(let j = 0; j < this.teams[i].completions.length; j++){
                        if (this.teams[i].completions[j] != null){
                            let date = new Date(this.teams[i].completions[j])
                            times.push(date)
                            team_score.push(this.teams[i].points[j])
                        }
                    }

                    team_score = this.cumulativeSum(team_score);
                    let trace = {
                        x: times,
                        y: team_score,
                        mode: 'lines+markers',
                        name: this.teams[i].name,
                        marker: {
                            color: this.colorHash(this.teams[i].name + this.teams[i].id),
                        },
                        line: {
                            color: this.colorHash(this.teams[i].name + this.teams[i].id),
                        }
                    };
                    this.traces.push(trace);
                }

                this.traces.sort(function(a, b) {
                    var scorediff = b['y'][b['y'].length - 1] - a['y'][a['y'].length - 1];
                    if(!scorediff) {
                        return a['x'][a['x'].length - 1] - b['x'][b['x'].length - 1];
                    }
                    return scorediff;
                });
            },
            cumulativeSum: function (arr) {
                let result = arr.concat();
                for (let i = 0; i < arr.length; i++){
                    result[i] = arr.slice(0, i + 1).reduce(function(p, i){ return p + i; });
                }
                return result
            },
            colorHash: function (str) {
                let hash = 0;
                for (let i = 0; i < str.length; i++) {
                    hash = str.charCodeAt(i) + ((hash << 5) - hash);
                }
                let colour = '#';
                for (let j = 0; j < 3; j++) {
                    let value = (hash >> (j * 8)) & 0xFF;
                    colour += ('00' + value.toString(16)).substr(-2);
                }
                return colour;
            }
        }
    }
</script>

<style scoped>

</style>