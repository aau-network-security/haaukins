<template>
    <div>
        <b-modal ref="modal" id="challengeModal" centered hide-footer hide-header >
            <div class="modal-body">
                <button type="button" class="close" v-on:click="$bvModal.hide('challengeModal')">
                    <span aria-hidden="true">×</span>
                </button>
                <nav>
                    <div class="nav nav-tabs" id="nav-tab" role="tablist">
                        <a class="nav-item nav-link active" id="nav-challenge-tab" data-toggle="tab" href="#nav-challenge" role="tab" aria-controls="nav-challenge" aria-selected="true">Challenge</a>
                        <a class="nav-item nav-link" id="nav-solves-tab" data-toggle="tab" href="#nav-solves" role="tab" aria-controls="nav-solves" aria-selected="false">{{checkTeams(teamsCompleted)}} Solves</a>
                        <ResetChallenge v-if="!challenge.staticChallenge" :challengeTag="challenge.tag" v-on:resetChallenge="$emit('resetChallenge')"></ResetChallenge>
                        <RunChallenge v-if="!challenge.staticChallenge" :challengeTag="challenge.tag" v-on:runChallenge="$emit('runChallenge')" ></RunChallenge>
                    </div>
                </nav>
                <div class="tab-content">
                    <div class="tab-pane fade show active" id="nav-challenge" role="tabpanel" aria-labelledby="nav-challenge-tab">
                        <h2 class="chal-name text-center pt-5 pb-1">{{challenge.name}}</h2>
                        <h4 class="chal-value text-center mb-5">{{challenge.points}}</h4>
                        <span class="chal-desc mb-5">
                            <p v-html="challenge.teamDescription"></p>
                        </span>
                        <FlagChecker :challengeTag="challenge.tag" v-on:challengeComplete="$emit('challengeCompleteReload')" class="mt-5"></FlagChecker>
                    </div>
                    <div class="tab-pane fade" id="nav-solves" role="tabpanel" aria-labelledby="nav-solves-tab">
                        <table class="table table-striped text-center mt-4">
                            <thead class="thead-dark-custom">
                                <tr>
                                    <th><b>Name</b></th>
                                    <th><b>Date</b></th>
                                </tr>
                            </thead>
                            <tbody v-if="checkTeams(teamsCompleted) != 0">
                                <tr v-for="team in teamsCompleted" v-bind:key="team.teamName">
                                    <td>{{team.teamName}}</td>
                                    <td>{{beauty_date(team.completedAt)}}</td>
                                </tr>
                            </tbody>
                            <tbody v-else>
                                <tr><td colspan="2">Nobody solved this challenge!</td></tr>
                            </tbody>
                        </table>
                    </div>
                </div>
            </div>
        </b-modal>
    </div>
</template>

<script>
import FlagChecker from "./FlagChecker";
import ResetChallenge from "@/components/ResetChallenge";
import RunChallenge from "./RunChallenge";

export default {
        name: "ChallengeModal",
        components: {ResetChallenge, FlagChecker, RunChallenge},
        props: {
            challenge: Object,
            teamsCompleted: Array,
        },
        data: function (){
            return{
            }
        },
        methods: {
            checkTeams: function (teamsCompleted) {
                if (teamsCompleted != null) {
                    return teamsCompleted.length
                }
                return 0
            },
            beauty_date: function (input_date) {
                let date = new Date(input_date);
                const monthNames = ["January", "February", "March", "April", "May", "June",
                    "July", "August", "September", "October", "November", "December"
                ];
                return date.getHours() + ":" + date.getMinutes() + "   " + date.getDate() + " " + monthNames[date.getMonth()]
            }
        }
    }
</script>

<style scoped>
    a {
        color: #211A52;
        text-decoration: none;
    }
</style>